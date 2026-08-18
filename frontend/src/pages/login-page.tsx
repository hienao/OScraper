import { Lock, Login, User } from '@appica/icons-react'
import { Button } from '@appica/ui-react/button'
import { Input } from '@appica/ui-react/input'
import { useState, type FormEvent } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { FormField } from '@/components/common/form-field'
import { Message } from '@/components/common/message'
import { Panel } from '@/components/common/panel'
import { useAuth } from '@/features/auth/use-auth'
import { errorMessage } from '@/lib/error-message'

export function LoginPage() {
  const { user, login } = useAuth()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  if (user) return <Navigate to="/" replace />
  async function submit(event: FormEvent) {
    event.preventDefault()
    await login.mutateAsync({ username: username.trim(), password })
    navigate('/', { replace: true })
  }
  return (
    <div className="grid min-h-[calc(100vh-8rem)] place-items-center px-4 py-12">
      <Panel className="rise-in w-full max-w-md">
        <div className="mb-7 text-center"><span className="mx-auto grid size-14 place-items-center rounded-2xl bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"><Login size={28} /></span><h1 className="mt-4 text-2xl font-bold">{t('auth.login.title')}</h1><p className="mt-1 text-sm text-neutral-500">{t('auth.login.description')}</p></div>
        <form className="space-y-5" onSubmit={(event) => void submit(event)}>
          {login.error && <Message variant="error">{errorMessage(login.error, t('auth.login.failed'))}</Message>}
          <FormField label={t('auth.login.username')}><Input value={username} onChange={(event) => setUsername(event.target.value)} startSlot={<User size={17} />} autoComplete="username" required /></FormField>
          <FormField label={t('auth.login.password')}><Input type="password" value={password} onChange={(event) => setPassword(event.target.value)} startSlot={<Lock size={17} />} autoComplete="current-password" required /></FormField>
          <Button className="w-full justify-center" type="submit" size="lg" disabled={login.isPending}>{t(login.isPending ? 'auth.login.submitting' : 'auth.login.submit')}</Button>
        </form>
        <p className="mt-5 rounded-lg bg-amber-50 px-3 py-2 text-center text-xs text-amber-800 dark:bg-amber-950/30 dark:text-amber-200">{t('auth.login.hint')}</p>
      </Panel>
    </div>
  )
}
