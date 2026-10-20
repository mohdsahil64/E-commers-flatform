// useRazorpay lazily loads the Razorpay Checkout script and opens the hosted
// payment popup. The publishable key, order id, amount and currency all come
// from the backend's /pay response (payOrder), so the frontend never hardcodes
// a key and TEST vs LIVE is decided entirely by the backend's .env.
//
// Razorpay Checkout docs: https://razorpay.com/docs/payments/payment-gateway/web-integration/standard/
// Content from the Razorpay docs was rephrased for compliance.

const RAZORPAY_SCRIPT_SRC = 'https://checkout.razorpay.com/v1/checkout.js'

declare global {
  interface Window {
    Razorpay?: new (options: RazorpayOptions) => { open: () => void }
  }
}

// Handle returned by the backend's payOrder() call.
export interface GatewayOrder {
  provider_order_id: string
  amount: number // minor units (paise)
  currency: string
  key_id: string // Razorpay publishable key (rzp_test_* or rzp_live_*)
}

// Result passed back to the server for signature verification.
export interface RazorpayResult {
  provider_order_id: string
  provider_payment_id: string
  signature: string
}

interface RazorpayOptions {
  key: string
  amount: number
  currency: string
  order_id: string
  name?: string
  description?: string
  prefill?: { name?: string; email?: string; contact?: string }
  theme?: { color?: string }
  handler: (resp: RazorpayHandlerResponse) => void
  modal?: { ondismiss?: () => void }
}

interface RazorpayHandlerResponse {
  razorpay_order_id: string
  razorpay_payment_id: string
  razorpay_signature: string
}

let scriptPromise: Promise<void> | null = null

// loadRazorpayScript injects the Checkout script once and resolves when ready.
function loadRazorpayScript(): Promise<void> {
  if (typeof window !== 'undefined' && window.Razorpay) return Promise.resolve()
  if (scriptPromise) return scriptPromise
  scriptPromise = new Promise<void>((resolve, reject) => {
    const el = document.createElement('script')
    el.src = RAZORPAY_SCRIPT_SRC
    el.async = true
    el.onload = () => resolve()
    el.onerror = () => {
      scriptPromise = null
      reject(new Error('Could not load the Razorpay payment library. Check your connection and try again.'))
    }
    document.head.appendChild(el)
  })
  return scriptPromise
}

export interface OpenCheckoutArgs {
  order: GatewayOrder
  name?: string
  description?: string
  prefill?: { name?: string; email?: string; contact?: string }
  themeColor?: string
}

// openRazorpayCheckout opens the Razorpay popup for the given gateway order and
// resolves with the signature material on success, or rejects if the user
// dismisses the popup or the script fails to load.
export async function openRazorpayCheckout(args: OpenCheckoutArgs): Promise<RazorpayResult> {
  await loadRazorpayScript()
  if (!window.Razorpay) {
    throw new Error('Razorpay failed to initialise.')
  }
  const { order } = args
  return new Promise<RazorpayResult>((resolve, reject) => {
    let settled = false
    const rzp = new window.Razorpay!({
      key: order.key_id,
      amount: order.amount,
      currency: order.currency,
      order_id: order.provider_order_id,
      name: args.name || 'CARLITOS',
      description: args.description,
      prefill: args.prefill,
      theme: { color: args.themeColor || '#1c1917' },
      handler: (resp) => {
        settled = true
        resolve({
          provider_order_id: resp.razorpay_order_id,
          provider_payment_id: resp.razorpay_payment_id,
          signature: resp.razorpay_signature,
        })
      },
      modal: {
        ondismiss: () => {
          if (!settled) reject(new Error('Payment was cancelled.'))
        },
      },
    })
    rzp.open()
  })
}
