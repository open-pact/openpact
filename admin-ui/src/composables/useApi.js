import { useAuth } from './useAuth'

let isRefreshing = false
let failedQueue = []

const processQueue = (error, token = null) => {
  failedQueue.forEach(prom => {
    if (error) {
      prom.reject(error)
    } else {
      prom.resolve(token)
    }
  })
  failedQueue = []
}

export function useApi() {
  const auth = useAuth()

  async function request(url, options = {}) {
    const headers = {
      'Content-Type': 'application/json',
      ...auth.getAuthHeader(),
      ...options.headers,
    }

    const response = await fetch(url, {
      ...options,
      headers,
      credentials: 'include',
    })

    // Handle 401 - try to refresh token
    if (response.status === 401 && !url.includes('/auth/') && !url.includes('/session')) {
      if (isRefreshing) {
        // Queue this request
        return new Promise((resolve, reject) => {
          failedQueue.push({ resolve, reject })
        }).then(() => {
          // Retry with new token
          return request(url, options)
        })
      }

      isRefreshing = true

      try {
        const refreshed = await auth.refreshToken()
        if (refreshed) {
          processQueue(null)
          // Retry original request
          return request(url, options)
        } else {
          processQueue(new Error('Refresh failed'))
          window.location.href = '/login'
          throw new Error('Session expired')
        }
      } finally {
        isRefreshing = false
      }
    }

    return response
  }

  async function get(url) {
    return request(url, { method: 'GET' })
  }

  async function post(url, data) {
    return request(url, {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  async function put(url, data) {
    return request(url, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  }

  async function del(url) {
    return request(url, { method: 'DELETE' })
  }

  return { request, get, post, put, del }
}
