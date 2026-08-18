import { Activity } from '@appica/icons-react'
import { useTranslation } from 'react-i18next'
import { Panel } from '@/components/common/panel'

export function LogsPage() {
  const { t } = useTranslation()
  return <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8"><Panel title={t('navigation.logs')} description={t('logs.description')} icon={<Activity size={20} />}><code className="block rounded-xl bg-neutral-950 p-4 text-sm text-emerald-200">GET /api/admin/logs<br />GET /api/admin/application-logs<br />GET /api/admin/audit-logs</code></Panel></div>
}
