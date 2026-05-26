export type EmailVerificationPurpose = 'register' | 'change_email' | 'oidc_create_account'
export type PublicEmailVerificationPurpose = Extract<
  EmailVerificationPurpose,
  'register' | 'oidc_create_account'
>
export type AccountEmailVerificationPurpose = Exclude<EmailVerificationPurpose, 'register'>

export interface EmailVerificationStartResponse {
  status: string
  expires_at?: string
  resend_at?: string
  cooldown_seconds?: number
  resend_after_seconds?: number
}

export interface EmailVerificationConfirmResponse {
  status: string
  verification_token: string
  expires_at?: string
}
