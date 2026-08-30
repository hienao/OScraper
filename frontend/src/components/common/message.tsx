import { AlertCircle, AlertTriangle, CircleCheck } from '@appica/icons-react'
import { Alert, AlertDescription, AlertIcon } from '@appica/ui-react/alert'

export function Message({ variant, children }: { variant: 'error' | 'success' | 'warning'; children: React.ReactNode }) {
  const error = variant === 'error'
  const icon = error ? <AlertCircle size={17} /> : variant === 'warning' ? <AlertTriangle size={17} /> : <CircleCheck size={17} />
  return (
    <Alert variant={variant} role={error ? 'alert' : 'status'} className="p-3">
      <AlertIcon>{icon}</AlertIcon>
      <AlertDescription className="mt-0 min-w-0 break-words">{children}</AlertDescription>
    </Alert>
  )
}
