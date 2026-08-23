import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@appica/ui-react/select'
import type { ReactNode } from 'react'

export interface AppSelectOption {
  value: string
  label: string
  disabled?: boolean
}

export function AppSelect({
  value,
  onValueChange,
  options,
  ariaLabel,
  placeholder,
  className = '',
  startSlot,
}: {
  value: string
  onValueChange: (value: string) => void
  options: AppSelectOption[]
  ariaLabel: string
  placeholder?: string
  className?: string
  startSlot?: ReactNode
}) {
  return (
    <Select value={value} onValueChange={(next) => onValueChange(String(next))} alignItemWithTrigger={false}>
      <SelectTrigger className={`w-full ${className}`} aria-label={ariaLabel} startSlot={startSlot}>
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => <SelectItem key={option.value} value={option.value} disabled={option.disabled}>{option.label}</SelectItem>)}
      </SelectContent>
    </Select>
  )
}
