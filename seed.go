package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// legacyEnvKeys are the only variables read by the one-time seed. Infrastructure values are not part of it.
var legacyEnvKeys = []string{
	"NBCC_FEED_URL", "GRAPNEL_API_KEY", "GRAPNEL_INSTANCE_ID", "GRAPNEL_GROUP_JID", "GRAPNEL_GROUP_NAME",
	"WEBHOOK_SECRET", "SEND_ENABLED", "POLL_INTERVAL", "COURSE_INSTRUCTORS", "ADMIN_TOKEN",
}

// readDotEnv parses KEY=value lines. A missing file is not an error.
func readDotEnv(path string) (map[string]string, error) {
	values := map[string]string{}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), string(rune(0xFEFF))))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, errors.New("invalid .env line")
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), "\"'")
	}
	return values, scanner.Err()
}

// legacyValues merges environment variables (which win) with the optional .env file.
// A missing or unreadable .env never fails startup; only its absence is silent.
func legacyValues(envFile string) map[string]string {
	dotenv, err := readDotEnv(envFile)
	if err != nil {
		log.Printf("legacy .env ignored: it could not be read")
		dotenv = map[string]string{}
	}
	values := map[string]string{}
	for _, key := range legacyEnvKeys {
		if v, ok := os.LookupEnv(key); ok {
			values[key] = v
		} else if v, ok := dotenv[key]; ok {
			values[key] = v
		}
	}
	return values
}

// seedSettings maps the legacy configuration onto settings. Each field is validated with the same rules as
// the admin API; an invalid legacy value is skipped and reported by field name only, never by value.
func seedSettings(envFile string) Settings {
	values := legacyValues(envFile)
	s := defaultSettings()
	var imported []string
	apply := func(field string, v any) {
		msg, err := json.Marshal(v)
		if err != nil {
			log.Printf("legacy setting %s skipped: invalid value", field)
			return
		}
		next, errs := applyPatch(s, map[string]json.RawMessage{field: msg})
		if len(errs) > 0 {
			log.Printf("legacy setting %s skipped: %s; configure it in /admin", field, errs[field])
			return
		}
		s = next
		imported = append(imported, field)
	}
	text := func(env string) (string, bool) {
		v := strings.TrimSpace(values[env])
		return v, v != ""
	}
	if v, ok := text("NBCC_FEED_URL"); ok {
		apply("nbccFeedUrl", v)
	}
	if v, ok := text("GRAPNEL_API_KEY"); ok {
		apply("grapnelApiKey", v)
	}
	if v, ok := text("GRAPNEL_INSTANCE_ID"); ok {
		apply("grapnelInstanceId", v)
	}
	if v, ok := text("GRAPNEL_GROUP_JID"); ok {
		apply("grapnelGroupJid", v)
	}
	if v, ok := text("GRAPNEL_GROUP_NAME"); ok {
		apply("grapnelGroupName", v)
	}
	if v, ok := text("WEBHOOK_SECRET"); ok {
		apply("webhookSecret", v)
	}
	if v, ok := text("SEND_ENABLED"); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			apply("sendEnabled", b)
		} else {
			log.Printf("legacy setting sendEnabled skipped: invalid value; configure it in /admin")
		}
	}
	if v, ok := text("POLL_INTERVAL"); ok {
		if d, err := time.ParseDuration(v); err == nil {
			minutes := int(d / time.Minute)
			if minutes < 1 {
				minutes = 1
			}
			apply("pollIntervalMinutes", minutes)
		} else {
			log.Printf("legacy setting pollIntervalMinutes skipped: invalid value; configure it in /admin")
		}
	}
	if v, ok := values["COURSE_INSTRUCTORS"]; ok && strings.TrimSpace(v) != "" {
		if m, err := parseCourseInstructors(v); err == nil {
			apply("courseInstructors", m)
		} else {
			log.Printf("legacy setting courseInstructors skipped: %v; configure it in /admin", err)
		}
	}
	if s.SendEnabled && !s.grapnelSendReady() {
		log.Printf("legacy setting sendEnabled ignored: Grapnel configuration is incomplete")
		s.SendEnabled = false
	}
	if strings.TrimSpace(values["ADMIN_TOKEN"]) != "" {
		log.Printf("ADMIN_TOKEN is no longer used and was ignored; the admin area now uses /admin")
	}
	if len(imported) > 0 {
		log.Printf("legacy settings imported once: %s", strings.Join(imported, ", "))
	} else {
		log.Printf("no legacy settings found; starting with defaults")
	}
	return s
}

// parseCourseInstructors accepts "Course code=Instructor name" pairs separated by semicolons.
// Errors name the entry position only, never its contents.
func parseCourseInstructors(s string) (map[string]string, error) {
	instructors := map[string]string{}
	for i, pair := range strings.Split(s, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		course, instructor, ok := strings.Cut(pair, "=")
		course, instructor = strings.TrimSpace(course), strings.TrimSpace(instructor)
		if !ok || course == "" || instructor == "" {
			return nil, fmt.Errorf("COURSE_INSTRUCTORS entry %d must be Course code=Instructor name", i+1)
		}
		if _, dup := instructors[course]; dup {
			return nil, fmt.Errorf("COURSE_INSTRUCTORS entry %d repeats a course code", i+1)
		}
		instructors[course] = instructor
	}
	return instructors, nil
}
