import { createIdempotencyKey } from './idempotency-key'

describe('createIdempotencyKey', () => {
  it('uses randomUUID when it is available', () => {
    const randomUUID = vi.fn(() => 'd9428888-122b-4c26-a04f-efc2b3e153a7')
    const getRandomValues = vi.fn((bytes: Uint8Array) => bytes)

    expect(createIdempotencyKey({ randomUUID, getRandomValues })).toBe('d9428888-122b-4c26-a04f-efc2b3e153a7')
    expect(getRandomValues).not.toHaveBeenCalled()
  })

  it('creates a version 4 UUID with getRandomValues when randomUUID is unavailable', () => {
    const getRandomValues = vi.fn((bytes: Uint8Array) => {
      bytes.set(Array.from({ length: 16 }, (_, index) => index))
      return bytes
    })

    expect(createIdempotencyKey({ getRandomValues })).toBe('00010203-0405-4607-8809-0a0b0c0d0e0f')
  })
})
