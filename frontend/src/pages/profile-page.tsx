import { ShieldCheck, User } from '@appica/icons-react'
import { useTranslation } from 'react-i18next'
import { Panel } from '@/components/common/panel'
import { useAuth } from '@/features/auth/use-auth'

export function ProfilePage() {
  const { user } = useAuth()
  const { t } = useTranslation()
  return <div className="mx-auto max-w-3xl px-4 py-8 sm:px-6 lg:px-8"><Panel title={t('profile.title')} description={t('profile.description')} icon={<User size={20} />}><dl className="grid gap-5 sm:grid-cols-2"><div><dt className="text-sm text-neutral-500">{t('profile.username')}</dt><dd className="mt-1 font-semibold">{user?.username}</dd></div><div><dt className="text-sm text-neutral-500">{t('profile.role')}</dt><dd className="mt-1 flex items-center gap-2 font-semibold"><ShieldCheck size={17} />{t('navigation.administrator')}</dd></div></dl></Panel></div>
}
