const storageKey = 'openlist-scraper-access-token'

export function readAccessToken() { return window.localStorage.getItem(storageKey) }
export function writeAccessToken(token: string) { window.localStorage.setItem(storageKey, token) }
export function removeAccessToken() { window.localStorage.removeItem(storageKey) }
