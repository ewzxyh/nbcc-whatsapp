package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

const grapnelBase = "https://grapnel.com.br/api/v1"

type apiError struct {
	Status     int
	Code       string
	RetryAfter int
}

func (e *apiError) Error() string { return fmt.Sprintf("Grapnel HTTP %d (%s)", e.Status, e.Code) }

func (a *App) api(ctx context.Context, method, path string, body any, key string, out any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	r, err := http.NewRequestWithContext(ctx, method, grapnelBase+path, bytes.NewReader(data))
	if err != nil {
		return errors.New("invalid Grapnel request")
	}
	r.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	res, err := a.client.Do(r)
	if err != nil {
		return errors.New("Grapnel network error")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var problem struct {
			Code string `json:"code"`
		}
		json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&problem)
		retry, _ := strconv.Atoi(res.Header.Get("Retry-After"))
		return &apiError{res.StatusCode, cleanText(problem.Code), retry}
	}
	if out != nil {
		if err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(out); err != nil {
			return errors.New("invalid Grapnel JSON response")
		}
	}
	return nil
}

type Group struct {
	JID     string `json:"jid"`
	Name    string `json:"name"`
	CanSend *bool  `json:"owner_can_send_message,omitempty"`
}

func groupName(s string) string {
	return strings.Join(strings.Fields(strings.TrimFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })), " ")
}

func (a *App) discover(ctx context.Context) (Group, error) {
	if a.cfg.InstanceID == "" {
		return Group{}, errors.New("GRAPNEL_INSTANCE_ID is required")
	}
	var result struct {
		Groups []Group `json:"groups"`
	}
	if err := a.api(ctx, "GET", "/instances/"+a.cfg.InstanceID+"/groups", nil, "", &result); err != nil {
		return Group{}, err
	}
	var found []Group
	for _, g := range result.Groups {
		if a.cfg.GroupJID != "" {
			if g.JID == a.cfg.GroupJID {
				found = append(found, g)
			}
		} else if strings.EqualFold(groupName(g.Name), groupName(a.cfg.GroupName)) {
			found = append(found, g)
		}
	}
	if len(found) != 1 {
		return Group{}, errors.New("expected exactly one matching WhatsApp group")
	}
	if !groupJIDPattern.MatchString(found[0].JID) {
		return Group{}, errors.New("invalid group identifier in API response")
	}
	return found[0], nil
}

func (a *App) sendText(ctx context.Context, text, key string) error {
	if !a.cfg.SendEnabled {
		return errors.New("sending is disabled")
	}
	if len(utf16.Encode([]rune(text))) > 4096 {
		return errors.New("agenda exceeds the WhatsApp message limit")
	}
	if a.cfg.GroupJID == "" {
		return errors.New("target group is not configured")
	}
	return a.api(ctx, "POST", "/instances/"+a.cfg.InstanceID+"/messages/text", map[string]string{"number": a.cfg.GroupJID, "text": text}, "nbcc:"+key, nil)
}

func messageParts(text string) []string {
	var parts []string
	for len(text) > 0 {
		units, end, lastLine := 0, 0, 0
		for end < len(text) {
			r, size := utf8.DecodeRuneInString(text[end:])
			if units+utf16.RuneLen(r) > 4096 {
				break
			}
			units += utf16.RuneLen(r)
			end += size
			if r == '\n' {
				lastLine = end
			}
		}
		if end < len(text) && lastLine > 0 {
			end = lastLine
		}
		part := strings.TrimSpace(text[:end])
		if part != "" {
			parts = append(parts, part)
		}
		text = text[end:]
	}
	return parts
}

func (a *App) registerWebhook(ctx context.Context, destination string) error {
	u, err := url.Parse(destination)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Port() != "" && u.Port() != "443" {
		return errors.New("a public HTTPS webhook URL on port 443 is required")
	}
	var current struct {
		Webhook *struct {
			URL string `json:"url"`
		} `json:"webhook"`
	}
	path := "/instances/" + a.cfg.InstanceID + "/groups/webhook"
	if err = a.api(ctx, "GET", path, nil, "", &current); err != nil {
		return err
	}
	if current.Webhook != nil {
		return errors.New("an instance webhook is already configured; refusing to replace it")
	}
	return a.api(ctx, "POST", path, map[string]string{"url": destination, "secret": a.cfg.WebhookSecret}, "nbcc-webhook-"+randomID(), nil)
}
