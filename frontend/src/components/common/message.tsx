import { AlertCircle, CircleCheck } from '@appica/icons-react'
import { Alert, AlertDescription, AlertIcon } from '@appica/ui-react/alert'

export function Message({ variant, children }: { variant: 'error' | 'success'; children: React.ReactNode }) {
  const error = variant === 'error'
  return (
    <Alert variant={variant} role={error ? 'alert' : 'status'} className="p-3">
      <AlertIcon>{error ? <AlertCircle size={17} /> : <CircleCheck size={17} />}</AlertIcon>
      <AlertDescription className="mt-0">{children}</AlertDescription>
    </Alert>
  )
}
