import { Lock, ShieldCheck, User } from '@appica/icons-react'
import { Button } from '@appica/ui-react/button'
import { Input } from '@appica/ui-react/input'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/features/auth/use-auth'
import { errorMessage } from '@/lib/error-message'
import { AppDialog } from './app-dialog'
import { FormField } from './form-field'
import { Message } from './message'

export function AdminSetupDialog() {
  const { user, setupAdmin } = useAuth()
  const { t } = useTranslation()
  const [username, setUsername] = useState(user?.username ?? 'admin')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  if (!user?.requires_admin_setup) return null
  async function submit(event: FormEvent) {
    event.preventDefault()
    if (password !== confirm) return
    await setupAdmin.mutateAsync({ username: username.trim(), password })
  }
  const mismatch = confirm !== '' && password !== confirm
  return (
    <AppDialog open onOpenChange={() => {}} width="sm" closeLabel={t('common.close')} closeDisabled title={<span className="flex items-center gap-3"><span className="grid size-10 place-items-center rounded-xl bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"><ShieldCheck size={21} /></span>{t('auth.setup.title')}</span>} description={t('auth.setup.description')} onSubmit={(event) => void submit(event)} bodyClassName="space-y-4" footer={<Button className="w-full justify-center" type="submit" size="lg" disabled={mismatch || setupAdmin.isPending}>{t('auth.setup.submit')}</Button>}>
      {setupAdmin.error && <Message variant="error">{errorMessage(setupAdmin.error, t('auth.setup.failed'))}</Message>}
      <FormField label={t('auth.setup.username')}><Input value={username} onChange={(event) => setUsername(event.target.value)} startSlot={<User size={17} />} required minLength={3} maxLength={50} /></FormField>
      <FormField label={t('auth.setup.password')}><Input type="password" value={password} onChange={(event) => setPassword(event.target.value)} startSlot={<Lock size={17} />} required minLength={8} autoComplete="new-password" /></FormField>
      <FormField label={t('auth.setup.confirm')} error={mismatch ? t('auth.setup.mismatch') : undefined}><Input type="password" value={confirm} onChange={(event) => setConfirm(event.target.value)} startSlot={<Lock size={17} />} required minLength={8} autoComplete="new-password" /></FormField>
    </AppDialog>
  )
}
