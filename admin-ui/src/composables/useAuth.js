import { ref, computed } from 'vue'

// Shared state
const accessToken = ref(null)
const user = ref(null)
const tokenExpiry = ref(null)

export function useAuth() {
  const isAuthenticated = computed(() => !!accessToken.value && tokenExpiry.value > Date.now())

  async function login(username, password) {
    const response = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
      credentials: 'include', // Important for cookies
    })

    if (!response.ok) {
      const data = await response.json()
      throw new Error(data.message || 'Login failed')
    }

    // Login sets the refresh cookie, now get access token
    return await refreshToken()
  }

  async function refreshToken() {
    try {
      const response = await fetch('/api/session', {
        credentials: 'include', // Send refresh cookie
      })

      if (!response.ok) {
        accessToken.value = null
        user.value = null
        tokenExpiry.value = null
        return false
      }

      const data = await response.json()
      accessToken.value = data.access_token
      user.value = { username: data.username }
      tokenExpiry.value = new Date(data.expires_at).getTime()
      return true
    } catch (e) {
      accessToken.value = null
      user.value = null
      tokenExpiry.value = null
      return false
    }
  }

  async function logout() {
    try {
      await fetch('/api/auth/logout', {
        method: 'POST',
        credentials: 'include',
      })
    } catch (e) {
      // Ignore errors
    }
    accessToken.value = null
    user.value = null
    tokenExpiry.value = null
  }

  function getAuthHeader() {
    return accessToken.value ? { Authorization: `Bearer ${accessToken.value}` } : {}
  }

  return {
    accessToken,
    user,
    isAuthenticated,
    login,
    logout,
    refreshToken,
    getAuthHeader,
  }
}
