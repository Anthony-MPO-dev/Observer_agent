import { useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

const TOKEN_KEY = 'logstream_token'

export function useAuth() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY))
  const navigate = useNavigate()

  const login = useCallback(async (username: string, password: string): Promise<void> => {
    const resp = await api.login(username, password)
    localStorage.setItem(TOKEN_KEY, resp.token)
    setToken(resp.token)
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY)
    setToken(null)
    navigate('/login', { replace: true })
  }, [navigate])

  return {
    token,
    isAuthenticated: !!token,
    login,
    logout,
  }
}
