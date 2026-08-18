import { Moon, Sun } from '@appica/icons-react'
import { Button } from '@appica/ui-react/button'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

export function ThemeToggle() {
  const [dark, setDark] = useState(false)
  const { t } = useTranslation()
  useEffect(() => setDark(document.documentElement.classList.contains('dark')), [])
  function toggle() {
    const next = !dark
    document.documentElement.classList.toggle('dark', next)
    document.documentElement.style.colorScheme = next ? 'dark' : 'light'
    localStorage.setItem('openlist-scraper-theme', next ? 'dark' : 'light')
    setDark(next)
  }
  return <Button variant="ghost" size="icon-md" aria-label={t(dark ? 'theme.light' : 'theme.dark')} onClick={toggle}>{dark ? <Sun size={18} /> : <Moon size={18} />}</Button>
}
