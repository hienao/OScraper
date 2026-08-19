import { Field, FieldDescription, FieldError, FieldLabel } from '@appica/ui-react/field'
import type { ReactNode } from 'react'

export function FormField({ label, description, error, children }: { label: string; description?: string; error?: string; children: ReactNode }) {
  return (
    <Field invalid={Boolean(error)} className="space-y-2">
      <FieldLabel>{label}</FieldLabel>
      {children}
      {description && !error && <FieldDescription>{description}</FieldDescription>}
      {error && <FieldError match>{error}</FieldError>}
    </Field>
  )
}
