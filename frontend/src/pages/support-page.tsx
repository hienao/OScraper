import { Bug, Coffee, HeartSpark, Qrcode, Server, Share } from '@appica/icons-react'
import { Badge } from '@appica/ui-react/badge'
import { useTranslation } from 'react-i18next'
import alipaySponsor from '@/assets/support/alipay-sponsor.png'
import wechatSponsor from '@/assets/support/wechat-sponsor.png'
import { Panel } from '@/components/common/panel'

const methods = [
  { key: 'wechat', image: wechatSponsor, mark: '微', markClass: 'bg-emerald-600 text-white', badgeClass: 'border-emerald-200 text-emerald-700 dark:border-emerald-800 dark:text-emerald-300' },
  { key: 'alipay', image: alipaySponsor, mark: '支', markClass: 'bg-blue-600 text-white', badgeClass: 'border-blue-200 text-blue-700 dark:border-blue-800 dark:text-blue-300' },
] as const

const contributions = [
  { key: 'issues', icon: Bug },
  { key: 'sharing', icon: Share },
] as const

export function SupportPage() {
  const { t } = useTranslation()

  return <div className="mx-auto max-w-6xl space-y-5 px-4 py-8 sm:px-6 lg:px-8">
    <section className="relative overflow-hidden rounded-3xl border border-emerald-200/70 bg-gradient-to-br from-white via-emerald-50/70 to-cyan-50 p-6 shadow-sm dark:border-emerald-900/70 dark:from-neutral-950 dark:via-emerald-950/35 dark:to-cyan-950/25 sm:p-8 lg:p-10">
      <span aria-hidden className="absolute -right-20 -top-20 size-64 rounded-full bg-emerald-300/20 blur-3xl dark:bg-emerald-500/10" />
      <span aria-hidden className="absolute -bottom-28 left-1/3 size-72 rounded-full bg-cyan-300/20 blur-3xl dark:bg-cyan-500/10" />
      <div className="relative grid gap-8 lg:grid-cols-[minmax(0,1.45fr)_minmax(280px,0.75fr)] lg:items-stretch">
        <div className="min-w-0">
          <p className="flex items-center gap-2 text-xs font-bold uppercase tracking-[0.18em] text-emerald-700 dark:text-emerald-300"><HeartSpark size={17} />{t('support.eyebrow')}</p>
          <h1 className="mt-4 max-w-3xl text-3xl font-bold tracking-tight text-neutral-950 dark:text-white sm:text-4xl">{t('support.title')}</h1>
          <p className="mt-4 max-w-3xl text-sm leading-7 text-neutral-600 dark:text-neutral-300 sm:text-base">{t('support.description')}</p>
          <div className="mt-6 flex flex-wrap gap-2">
            <Badge variant="outline" className="gap-1.5"><Server size={14} />{t('support.tags.compatibility')}</Badge>
            <Badge variant="outline" className="gap-1.5"><Qrcode size={14} />{t('support.tags.testing')}</Badge>
            <Badge variant="outline" className="gap-1.5"><HeartSpark size={14} />{t('support.tags.maintenance')}</Badge>
          </div>
        </div>
        <blockquote className="flex min-w-0 flex-col justify-between rounded-2xl border border-emerald-200/70 bg-white/75 p-5 shadow-sm backdrop-blur dark:border-emerald-900/70 dark:bg-neutral-950/65 sm:p-6">
          <span className="font-serif text-4xl leading-none text-emerald-600">“</span>
          <p className="mt-5 text-sm font-medium leading-7 text-neutral-800 dark:text-neutral-200">{t('support.quote')}</p>
          <footer className="mt-6 text-xs text-neutral-500">{t('support.thanks')}</footer>
        </blockquote>
      </div>
    </section>

    <div className="grid gap-5 lg:grid-cols-2">
      {methods.map((method) => <Panel key={method.key} title={t(`support.methods.${method.key}.title`)} description={t(`support.methods.${method.key}.description`)} icon={<span className={`grid size-9 place-items-center rounded-xl text-sm font-bold ${method.markClass}`}>{method.mark}</span>} action={<Badge variant="outline" className={method.badgeClass}>{t(`support.methods.${method.key}.badge`)}</Badge>}>
        <div className="mx-auto flex aspect-[2/3] w-full max-w-64 items-center justify-center overflow-hidden rounded-2xl border border-neutral-200 bg-white p-2 shadow-sm dark:border-neutral-700">
          <img className="block max-h-full w-full rounded-xl object-contain" src={method.image} alt={t(`support.methods.${method.key}.imageAlt`)} />
        </div>
        <p className="mx-auto mt-4 max-w-md text-center text-sm leading-6 text-neutral-500">{t(`support.methods.${method.key}.note`)}</p>
      </Panel>)}
    </div>

    <Panel title={t('support.other.title')} description={t('support.other.description')} icon={<Coffee size={20} />}>
      <div className="grid gap-3 sm:grid-cols-2">
        {contributions.map(({ key, icon: Icon }) => <article key={key} className="flex min-w-0 items-start gap-3 rounded-2xl bg-neutral-50 p-4 dark:bg-neutral-900/70"><span className="grid size-10 shrink-0 place-items-center rounded-xl bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"><Icon size={19} /></span><div className="min-w-0"><h3 className="font-semibold">{t(`support.other.${key}.title`)}</h3><p className="mt-1 text-sm leading-6 text-neutral-500">{t(`support.other.${key}.description`)}</p></div></article>)}
      </div>
      <p className="mt-5 rounded-2xl border border-emerald-200/70 bg-emerald-50/70 px-4 py-3 text-sm leading-6 text-emerald-900 dark:border-emerald-900/70 dark:bg-emerald-950/40 dark:text-emerald-200">{t('support.voluntary')}</p>
    </Panel>
  </div>
}
