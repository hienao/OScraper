import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@appica/ui-react/dialog'
import type { FormEvent, ReactNode } from 'react'

const widths = {
  sm: 'w-[36rem]',
  md: 'w-[48rem]',
  lg: 'w-[56rem]',
  xl: 'w-[64rem]',
} as const

export function AppDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  width = 'md',
  closeLabel,
  closeDisabled = false,
  bodyClassName = '',
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  description?: ReactNode
  children: ReactNode
  footer?: ReactNode
  width?: keyof typeof widths
  closeLabel: string
  closeDisabled?: boolean
  bodyClassName?: string
  onSubmit?: (event: FormEvent<HTMLFormElement>) => void
}) {
  const content = (
    <>
      <DialogHeader className="pe-16">
        <DialogTitle className="break-words text-xl">{title}</DialogTitle>
        {description && <DialogDescription className="break-words">{description}</DialogDescription>}
      </DialogHeader>
      <DialogBody className={`overflow-x-hidden overflow-y-auto pb-1 ${bodyClassName}`}>{children}</DialogBody>
      {footer && <DialogFooter>{footer}</DialogFooter>}
    </>
  )

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next && closeDisabled) return; onOpenChange(next) }}>
      <DialogContent className={`min-w-0 max-w-[calc(100vw-2rem)] ${widths[width]}`} closeButton={!closeDisabled} closeLabel={closeLabel}>
        {onSubmit ? <form className="flex min-h-0 flex-1 flex-col overflow-hidden" onSubmit={onSubmit}>{content}</form> : content}
      </DialogContent>
    </Dialog>
  )
}
