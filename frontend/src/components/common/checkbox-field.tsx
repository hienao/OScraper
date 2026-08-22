import { Checkbox } from '@appica/ui-react/checkbox'
import { Field, FieldDescription, FieldLabel } from '@appica/ui-react/field'

export function CheckboxField({
  checked,
  onCheckedChange,
  label,
  description,
}: {
  checked: boolean
  onCheckedChange: (checked: boolean) => void
  label: string
  description?: string
}) {
  return (
    <Field className="flex items-start gap-3 rounded-xl border border-neutral-200 p-3 dark:border-neutral-800">
      <Checkbox className="mt-1 text-base" checked={checked} onCheckedChange={(next) => onCheckedChange(Boolean(next))} aria-label={label} />
      <div className="min-w-0">
        <FieldLabel className="mb-0">{label}</FieldLabel>
        {description && <FieldDescription className="mt-1 text-xs">{description}</FieldDescription>}
      </div>
    </Field>
  )
}
