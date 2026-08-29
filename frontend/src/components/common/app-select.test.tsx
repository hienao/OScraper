import { render, screen } from '@testing-library/react'
import { AppSelect } from './app-select'

beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
})

describe('AppSelect', () => {
  it('shows the selected option label instead of its stored value', () => {
    render(
      <AppSelect
        value="42"
        onValueChange={() => {}}
        ariaLabel="OpenList connection"
        options={[{ value: '42', label: '家庭媒体库' }]}
      />,
    )

    expect(screen.getByRole('combobox')).toHaveTextContent('家庭媒体库')
    expect(screen.getByRole('combobox')).not.toHaveTextContent('42')
  })
})
