import { describe, expect, it } from 'vitest'
import { ApiError } from './api'
import { FALLBACK_MESSAGE, messageForCode, messageForError } from './errors'

const CASES: [string, string][] = [
  ['required', 'Preencha este campo.'],
  ['invalid_feed_url', 'Use o link HTTPS do calendário NBCC, copiado do Brightspace.'],
  ['invalid_uuid', 'Informe um ID no formato UUID.'],
  ['invalid_group_jid', 'Informe um ID de grupo no formato 000000000000@g.us.'],
  ['too_long', 'Este texto é longo demais.'],
  ['invalid_https_url', 'Use um endereço HTTPS, sem caminho ou parâmetros.'],
  ['out_of_range', 'Informe um número dentro do intervalo indicado.'],
  ['invalid_instructor_map', 'Revise os nomes dos professores.'],
  [
    'missing_grapnel_config',
    'Para enviar mensagens, configure a chave, o ID da instância e o ID do grupo.',
  ],
  ['too_short', 'A senha precisa ter pelo menos 12 caracteres.'],
  ['same_as_email', 'A senha não pode ser igual ao seu e-mail.'],
  ['invalid_credentials', 'E-mail ou senha incorretos.'],
  ['too_many_attempts', 'Muitas tentativas. Aguarde alguns minutos e tente de novo.'],
  ['csrf', 'Sua sessão precisa ser atualizada. Recarregue a página.'],
  ['unauthenticated', 'Sua sessão expirou. Entre novamente.'],
  ['session_expired', 'Sua sessão expirou. Entre novamente.'],
  ['feed_unavailable', 'Não foi possível ler o calendário NBCC agora.'],
  ['grapnel_unavailable', 'Não foi possível conectar ao Grapnel agora.'],
  ['not_configured', 'Salve as configurações necessárias antes de testar.'],
  [
    'webhook_already_configured',
    'Já existe um webhook registrado no Grapnel. Ele não foi substituído.',
  ],
  ['invalid_setup_code', 'Código de configuração incorreto.'],
  ['already_configured', 'O acesso já foi configurado. Entre com sua senha.'],
]

describe('messageForCode', () => {
  it.each(CASES)('maps %s to its pt-BR copy', (code, text) => {
    expect(messageForCode(code)).toBe(text)
  })

  it('falls back to the generic message for an unknown code', () => {
    expect(messageForCode('something_new')).toBe(FALLBACK_MESSAGE)
    expect(FALLBACK_MESSAGE).toBe('Algo deu errado. Tente novamente.')
  })

  it('does not treat inherited object keys as codes', () => {
    expect(messageForCode('constructor')).toBe(FALLBACK_MESSAGE)
    expect(messageForCode('__proto__')).toBe(FALLBACK_MESSAGE)
  })

  it('uses field-specific copy where the same code means something else', () => {
    expect(messageForCode('invalid_credentials', 'currentPassword')).toBe('Senha atual incorreta.')
    expect(messageForCode('invalid_credentials', 'email')).toBe('E-mail ou senha incorretos.')
    expect(messageForCode('too_short', 'webhookSecret')).toBe(
      'Use pelo menos 32 caracteres. Gere um novo segredo com o botão ao lado.',
    )
  })

  it('shows the password length message for the new password field', () => {
    expect(messageForCode('too_long', 'newPassword')).toBe(
      'A senha é longa demais. Use no máximo 256 caracteres.',
    )
  })
})

describe('messageForError', () => {
  it('uses the code of an ApiError', () => {
    expect(messageForError(new ApiError(401, 'invalid_credentials'))).toBe('E-mail ou senha incorretos.')
  })

  it('falls back for anything that is not an ApiError', () => {
    expect(messageForError(new Error('boom'))).toBe(FALLBACK_MESSAGE)
    expect(messageForError(undefined)).toBe(FALLBACK_MESSAGE)
  })
})
