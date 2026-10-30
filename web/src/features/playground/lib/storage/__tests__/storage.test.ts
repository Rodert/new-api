/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { afterEach, beforeEach, expect, test } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import { STORAGE_KEYS } from '../../../constants'
import type { Message } from '../../../types'
import {
  clearPlaygroundData,
  saveImageWorkspaceResult,
  saveVideoWorkspaceTask,
  loadImageWorkspaceItems,
  loadVideoWorkspaceTasks,
  loadConfig,
  loadMessages,
  loadParameterEnabled,
  saveConfig,
  saveMessages,
  saveParameterEnabled,
} from '../storage'

const messages: Message[] = [
  {
    key: 'a-chat',
    from: 'user',
    versions: [{ id: 'a-version', content: 'Account A private chat' }],
  },
]

beforeEach(() => {
  localStorage.clear()
  useAuthStore.getState().auth.reset()
})

afterEach(() => {
  localStorage.clear()
  useAuthStore.getState().auth.reset()
})

test('switching accounts isolates config, messages and parameter toggles and preserves returning account data', () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'A', role: 1 })
  saveConfig({ model: 'account-a-model' })
  saveMessages(messages)
  saveParameterEnabled({ temperature: true })

  useAuthStore.getState().auth.reset()
  useAuthStore.getState().auth.setUser({ id: 2, username: 'B', role: 1 })
  expect(loadConfig()).toEqual({})
  expect(loadMessages()).toBeNull()
  expect(loadParameterEnabled()).toEqual({})
  saveConfig({ model: 'account-b-model' })
  saveMessages([])
  saveParameterEnabled({ temperature: false })

  useAuthStore.getState().auth.setUser({ id: 1, username: 'A', role: 1 })
  expect(loadConfig()).toMatchObject({ model: 'account-a-model' })
  expect(loadMessages()).toEqual(messages)
  expect(loadParameterEnabled()).toMatchObject({ temperature: true })

  clearPlaygroundData()
  expect(loadMessages()).toEqual([])
  useAuthStore.getState().auth.setUser({ id: 2, username: 'B', role: 1 })
  expect(loadConfig()).toMatchObject({ model: 'account-b-model' })
  expect(loadMessages()).toEqual([])
  expect(loadParameterEnabled()).toMatchObject({ temperature: false })
})

test('legacy global data is removed instead of assigned to the first signed-in account', () => {
  localStorage.setItem(
    STORAGE_KEYS.CONFIG,
    JSON.stringify({ model: 'legacy-model' })
  )
  localStorage.setItem(STORAGE_KEYS.MESSAGES, JSON.stringify(messages))
  localStorage.setItem(
    STORAGE_KEYS.PARAMETER_ENABLED,
    JSON.stringify({ temperature: true })
  )
  useAuthStore.getState().auth.setUser({ id: 2, username: 'B', role: 1 })

  expect(loadConfig()).toEqual({})
  expect(loadMessages()).toBeNull()
  expect(loadParameterEnabled()).toEqual({})
  for (const key of Object.values(STORAGE_KEYS)) {
    expect(localStorage.getItem(key)).toBeNull()
  }
})

test('storage calls without a signed-in user do not persist shared data', () => {
  saveConfig({ model: 'signed-out-model' })
  saveMessages(messages)
  saveParameterEnabled({ temperature: true })
  expect(loadConfig()).toEqual({})
  expect(loadMessages()).toBeNull()
  expect(loadParameterEnabled()).toEqual({})
  expect(localStorage.length).toBe(0)
})

test('image and video histories retain all 25 generations until explicitly cleared', () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'A', role: 1 })
  for (let index = 0; index < 25; index++) {
    saveImageWorkspaceResult({
      data: [{ url: `https://example.com/${index}.png` }],
    })
    saveVideoWorkspaceTask({ id: `video-${index}`, status: 'queued' })
  }
  expect(loadImageWorkspaceItems()).toHaveLength(25)
  expect(loadImageWorkspaceItems()[24].data?.[0].url).toBe(
    'https://example.com/0.png'
  )
  expect(loadVideoWorkspaceTasks()).toHaveLength(25)
  expect(loadVideoWorkspaceTasks()[24].taskId).toBe('video-0')
  saveImageWorkspaceResult(null)
  saveVideoWorkspaceTask(null)
  expect(loadImageWorkspaceItems()).toEqual([])
  expect(loadVideoWorkspaceTasks()).toEqual([])
})

test('video percentage progress and null error preserve both histories and reference mode across reloads', () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'A', role: 1 })
  saveImageWorkspaceResult({ data: [{ url: 'https://example.com/image.png' }] })
  saveVideoWorkspaceTask(
    {
      id: 'video-reference',
      status: 'queued',
      progress: '0%',
      error: null,
      result: null,
    },
    {
      model: 'grok-image-video',
      group: 'default',
      prompt: 'animate',
      aspectRatio: '9:16',
      seconds: '10',
      grokVideoImageMode: 'reference',
      referenceImages: [
        'https://example.com/one.png',
        'https://example.com/two.png',
      ],
    }
  )
  expect(loadVideoWorkspaceTasks()).toMatchObject([
    {
      taskId: 'video-reference',
      progress: '0%',
      grokVideoImageMode: 'reference',
      referenceImages: [
        'https://example.com/one.png',
        'https://example.com/two.png',
      ],
    },
  ])
  expect(loadImageWorkspaceItems()).toHaveLength(1)
  saveVideoWorkspaceTask({
    id: 'video-reference',
    status: 'completed',
    progress: 100,
  })
  expect(loadVideoWorkspaceTasks()).toHaveLength(1)
  expect(loadImageWorkspaceItems()).toHaveLength(1)
})
