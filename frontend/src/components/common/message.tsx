import { AlertCircle, CircleCheck } from '@appica/icons-react'

export function Message({ variant, children }: { variant: 'error' | 'success'; children: React.ReactNode }) {
  const error = variant === 'error'
  return (
    <div role={error ? 'alert' : 'status'} className={`flex gap-2 rounded-xl border px-3 py-2.5 text-sm ${error ? 'border-red-200 bg-red-50 text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200' : 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200'}`}>
      {error ? <AlertCircle className="mt-0.5 shrink-0" size={17} /> : <CircleCheck className="mt-0.5 shrink-0" size={17} />}
      <span>{children}</span>
    </div>
  )
}
