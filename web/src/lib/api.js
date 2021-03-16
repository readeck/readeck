const csrfToken = document.querySelector('html>head>meta[name="x-csrf-token"]').content
const endpoint = document.querySelector('html>head>meta[name="x-api-url"]').content

const safeMethods = ["GET", "HEAD", "OPTIONS", "TRACE"]

const defaults = {
  baseURL: endpoint,
  method: "GET",
  redirect: "manual",
  headers: {},
}

// request calls fetch() for the unprefixed path and the given params.
// It adds the path prefix and some default headers such as the
// CSRF token on state change requests.
function request(path, params) {
  params = {...defaults, ...params}
  params.headers["Accept"] = "application/json"
  params.method = params.method ? params.method.toUpperCase() : "GET"
  if (!safeMethods.includes(params.method)) {
    params.headers["X-CSRF-Token"] = csrfToken
  }

  let url = new URL(path, `${endpoint}/`).pathname
  return fetch(url, params)
}

// getJSON returns the json response of an API call.
async function getJSON(path) {
  let rsp = await request(path)
  return rsp.json()
}

// patchJSON sends a POST request with data encoded in JSON
// and returns the JSON result.
async function postJSON(path, data) {
  let rsp = await request(path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  })
  return rsp.json()
}

// patchJSON sends a PATCH request with data encoded in JSON
// and returns the JSON result.
async function patchJSON(path, data) {
  let rsp = await request(path, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  })
  return rsp.json()
}

export default {
  available: window.fetch !== undefined,
  request,
  getJSON,
  postJSON,
  patchJSON,
}
