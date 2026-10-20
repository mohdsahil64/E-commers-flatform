// Temporary end-to-end verification for the COD + offers changes.
const API = 'http://localhost:8080/api/v1'

async function j(res) { const t = await res.text(); try { return JSON.parse(t) } catch { return t } }

async function login(path, body) {
  const r = await fetch(API + path, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
  })
  const d = await j(r)
  return d?.data?.access_token
}

const run = async () => {
  const ctok = await login('/auth/customer/login', { email: 'customer@carlitos.local', password: 'Pass@123' })
  console.log('customer token:', ctok ? 'OK' : 'FAIL')

  const coupons = await fetch(API + '/customer/coupons', { headers: { Authorization: 'Bearer ' + ctok } })
  const cd = await j(coupons)
  console.log('GET /customer/coupons:', coupons.status, 'count=', Array.isArray(cd?.data) ? cd.data.length : cd)

  const cart = await fetch(API + '/customer/cart', { headers: { Authorization: 'Bearer ' + ctok } })
  const cartd = await j(cart)
  const items = cartd?.data?.items || []
  console.log('cart items:', items.length, items.map(i => ({ seller: i.seller_name, cod: i.seller_cod_enabled })))

  const stok = await login('/auth/seller/login', { email: 'seller@carlitos.local', password: 'Pass@123' })
  console.log('seller token:', stok ? 'OK' : 'FAIL')

  const prof = await j(await fetch(API + '/seller/profile', { headers: { Authorization: 'Bearer ' + stok } }))
  console.log('seller cod_enabled (before):', prof?.data?.cod_enabled)

  // Toggle COD off
  const put = await fetch(API + '/seller/profile', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + stok },
    body: JSON.stringify({ cod_enabled: false }),
  })
  const putd = await j(put)
  console.log('PUT /seller/profile cod_enabled=false:', put.status, '=>', putd?.data?.cod_enabled)

  // Toggle back on
  const put2 = await fetch(API + '/seller/profile', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + stok },
    body: JSON.stringify({ cod_enabled: true }),
  })
  const put2d = await j(put2)
  console.log('PUT /seller/profile cod_enabled=true :', put2.status, '=>', put2d?.data?.cod_enabled)
}

run().catch(e => { console.error('ERROR', e); process.exit(1) })
