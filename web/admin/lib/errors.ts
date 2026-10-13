import { isApiError } from './api'

export const FALLBACK_MESSAGE = 'Algo deu errado. Tente novamente.'

const MESSAGES: Record<string, string> = {
  required: 'Preencha este campo.',
  invalid_email: 'Informe um e-mail válido.',
  fixed_admin_email: 'O e-mail de acesso é fixo e não pode ser alterado.',
  invalid_feed_url: 'Use o link HTTPS do calendário NBCC, copiado do Brightspace.',
  invalid_uuid: 'Informe um ID no formato UUID.',
  invalid_group_jid: 'Informe um ID de grupo no formato 000000000000@g.us.',
  too_long: 'Este texto é longo demais.',
  invalid_https_url: 'Use um endereço HTTPS, sem caminho ou parâmetros.',
  out_of_range: 'Informe um número dentro do intervalo indicado.',
  invalid_instructor_map: 'Revise os nomes dos professores.',
  missing_grapnel_config:
    'Para enviar mensagens, configure a chave, o ID da instância e o ID do grupo.',
  too_short: 'A senha precisa ter pelo menos 12 caracteres.',
  same_as_email: 'A senha não pode ser igual ao seu e-mail.',
  same_as_password: 'A nova senha precisa ser diferente da atual.',
  invalid_value: 'Valor inválido.',
  bad_request: 'Não foi possível processar o pedido. Tente novamente.',
  internal: 'Algo deu errado no servidor. Tente novamente em instantes.',
  invalid_credentials: 'E-mail ou senha incorretos.',
  too_many_attempts: 'Muitas tentativas. Aguarde alguns minutos e tente de novo.',
  csrf: 'Sua sessão precisa ser atualizada. Recarregue a página.',
  unauthenticated: 'Sua sessão expirou. Entre novamente.',
  session_expired: 'Sua sessão expirou. Entre novamente.',
  feed_unavailable: 'Não foi possível ler o calendário NBCC agora.',
  grapnel_unavailable: 'Não foi possível conectar ao Grapnel agora.',
  not_configured: 'Salve as configurações necessárias antes de testar.',
  webhook_already_configured: 'Já existe um webhook registrado no Grapnel. Ele não foi substituído.',
  invalid_setup_code: 'Código de configuração incorreto.',
  already_configured: 'O acesso já foi configurado. Entre com sua senha.',
  mismatch: 'As senhas não coincidem.',
  network: 'Não foi possível conectar ao servidor. Verifique sua conexão e tente de novo.',
}

// Same code, different copy depending on the field (for example a wrong current password).
const FIELD_MESSAGES: Record<string, string> = {
  'currentPassword:invalid_credentials': 'Senha atual incorreta.',
  'newPassword:too_long': 'A senha é longa demais. Use no máximo 256 caracteres.',
  'newEmail:same_as_password': 'O novo e-mail não pode ser igual à sua senha.',
  'password:too_long': 'A senha é longa demais. Use no máximo 256 caracteres.',
  'webhookSecret:too_short':
    'Use pelo menos 32 caracteres. Gere um novo segredo com o botão ao lado.',
}

export function messageForCode(code: string, field?: string): string {
  if (field && Object.hasOwn(FIELD_MESSAGES, `${field}:${code}`)) {
    return FIELD_MESSAGES[`${field}:${code}`]
  }
  return Object.hasOwn(MESSAGES, code) ? MESSAGES[code] : FALLBACK_MESSAGE
}

export function messageForError(error: unknown, field?: string): string {
  if (isApiError(error)) return messageForCode(error.code, field)
  return FALLBACK_MESSAGE
}
