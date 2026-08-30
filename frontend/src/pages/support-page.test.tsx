import { render, screen } from '@testing-library/react'
import { i18n } from '@/i18n'
import { navigation, titleKey } from '@/components/layout/navigation'
import { SupportPage } from './support-page'

beforeEach(async () => {
  await i18n.changeLanguage('zh-CN')
})

it('shows the support story, payment codes, and voluntary notice', () => {
  render(<SupportPage />)

  expect(screen.getByRole('heading', { level: 1, name: '如果 OScraper 替你省下了时间，欢迎支持它继续前进' })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: '微信赞赏码' })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: '支付宝收款码' })).toBeInTheDocument()
  expect(screen.getByRole('img', { name: '微信赞赏收款码' })).toHaveAttribute('src', expect.stringContaining('wechat-sponsor.png'))
  expect(screen.getByRole('img', { name: '支付宝赞赏收款码' })).toHaveAttribute('src', expect.stringContaining('alipay-sponsor.png'))
  expect(screen.getByText('赞赏完全自愿。OScraper 不会把功能与付费绑定，不赞赏也不会减少任何功能。')).toBeInTheDocument()
})

it('registers the support route in navigation metadata', () => {
  expect(navigation.some((item) => item.to === '/support' && item.label === 'navigation.support')).toBe(true)
  expect(titleKey('/support')).toBe('navigation.support')
})
