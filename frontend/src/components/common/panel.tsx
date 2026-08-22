import type { HTMLAttributes, ReactNode } from 'react'

interface PanelProps extends HTMLAttributes<HTMLElement> {
  title?: string
  description?: string
  icon?: ReactNode
  action?: ReactNode
}

export function Panel({ title, description, icon, action, children, className = '', ...props }: PanelProps) {
  return (
    <section className={`app-panel overflow-hidden ${className}`} {...props}>
      {(title || action) && (
        <header className="flex flex-col items-start gap-4 border-b border-neutral-200/70 px-5 py-4 dark:border-neutral-800/80 sm:flex-row sm:justify-between sm:px-6">
          <div className="flex min-w-0 items-start gap-3">
            {icon && <span className="mt-0.5 text-emerald-700 dark:text-emerald-400">{icon}</span>}
            <div className="min-w-0">
              {title && <h2 className="font-semibold tracking-tight text-neutral-950 dark:text-white">{title}</h2>}
              {description && <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">{description}</p>}
            </div>
          </div>
          {action && <div className="w-full min-w-0 sm:w-auto sm:shrink-0">{action}</div>}
        </header>
      )}
      <div className="p-5 sm:p-6">{children}</div>
    </section>
  )
}
