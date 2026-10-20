import { BRAND } from '@carlitos/shared'

const SUFFIX = `${BRAND}`
const DEFAULT_DESC =
  'Shop the latest fashion and lifestyle from trusted sellers on CARLITOS — a premium multi-vendor marketplace. Fast delivery, easy returns.'

export interface SeoInput {
  /** Page title WITHOUT the brand suffix (it's appended automatically). */
  title?: string
  description?: string
  /** Optional canonical path (defaults to current location). */
  canonicalPath?: string
}

function setMeta(name: string, content: string, attr: 'name' | 'property' = 'name') {
  if (typeof document === 'undefined') return
  let el = document.querySelector<HTMLMetaElement>(`meta[${attr}="${name}"]`)
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute(attr, name)
    document.head.appendChild(el)
  }
  el.setAttribute('content', content)
}

function setCanonical(href: string) {
  if (typeof document === 'undefined') return
  let el = document.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!el) {
    el = document.createElement('link')
    el.setAttribute('rel', 'canonical')
    document.head.appendChild(el)
  }
  el.setAttribute('href', href)
}

// useSeo centralizes per-page document metadata. Titles read like real
// marketplaces: "<Page> | CARLITOS". Product/seller/collection pages pass the
// entity name so the tab + share preview reflect the exact item.
export function useSeo(initial?: SeoInput) {
  function set(input: SeoInput) {
    const title = input.title ? `${input.title} | ${SUFFIX}` : `${SUFFIX} — Premium Fashion & Lifestyle Marketplace`
    const desc = input.description || DEFAULT_DESC
    if (typeof document !== 'undefined') {
      document.title = title
      setMeta('description', desc)
      setMeta('og:title', title, 'property')
      setMeta('og:description', desc, 'property')
      setMeta('twitter:title', title)
      setMeta('twitter:description', desc)
      const origin = window.location.origin
      const path = input.canonicalPath ?? window.location.pathname
      setCanonical(origin + path)
    }
  }

  if (initial) set(initial)

  return { set }
}
