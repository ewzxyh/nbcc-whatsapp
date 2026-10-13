export const FIELD_LABELS: Record<string, string> = {
  nbccFeedUrl: 'Link do calendário NBCC',
  grapnelApiKey: 'Chave de API do Grapnel',
  webhookSecret: 'Segredo do webhook',
  grapnelInstanceId: 'ID da instância do WhatsApp',
  grapnelGroupJid: 'ID do grupo do WhatsApp',
  grapnelGroupName: 'Nome do grupo',
  publicBaseUrl: 'Endereço público do serviço',
  sendEnabled: 'Enviar mensagens para o grupo',
  pollIntervalMinutes: 'Verificação do calendário (minutos)',
  agendaCacheMinutes: 'Atualização da agenda pública (minutos)',
  courseInstructors: 'Professores por disciplina',
  setupCode: 'Código de configuração',
  password: 'Senha',
  currentPassword: 'Senha atual',
  newPassword: 'Nova senha',
  confirmPassword: 'Confirmar nova senha',
  newEmail: 'Novo e-mail de acesso',
  email: 'E-mail de acesso',
}

export function fieldLabel(key: string): string {
  return Object.hasOwn(FIELD_LABELS, key) ? FIELD_LABELS[key] : key
}

export const OMISSION_REASON_LABELS: Record<string, string> = {
  unsupported_recurrence: 'Recorrência não compatível',
  recurrence_override: 'Alteração pontual de recorrência',
  high_frequency_recurrence: 'Repetição muito frequente',
  invalid_date: 'Data inválida',
  invalid_end: 'Horário de término inválido',
  invalid_title: 'Título inválido',
  invalid_location: 'Local inválido',
  duplicate_uid: 'Evento duplicado',
  other: 'Formato não compatível',
}

export function omissionReasonLabel(reason: string): string {
  return Object.hasOwn(OMISSION_REASON_LABELS, reason)
    ? OMISSION_REASON_LABELS[reason]
    : OMISSION_REASON_LABELS.other
}

export const AUDIT_ACTION_LABELS: Record<string, string> = {
  'setup.completed': 'Acesso configurado',
  'login.succeeded': 'Entrada realizada',
  'login.failed': 'Tentativa de entrada recusada',
  logout: 'Saída',
  'password.changed': 'Senha alterada',
  'email.changed': 'E-mail de acesso alterado',
  'settings.updated': 'Configurações alteradas',
  'webhook.registered': 'Webhook registrado',
  'sessions.revoked': 'Outras sessões encerradas',
}

export function auditActionLabel(action: string): string {
  return Object.hasOwn(AUDIT_ACTION_LABELS, action) ? AUDIT_ACTION_LABELS[action] : 'Ação registrada'
}

export function plural(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`
}
