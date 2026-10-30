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
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { DEFAULT_CONFIG } from '../../../constants'
import {
  loadVideoWorkspaceTasks,
  saveVideoWorkspaceTask,
} from '../../../lib/storage/storage'
import { VideoWorkspace } from '../video-workspace'

beforeEach(() => {
  localStorage.clear()
  useAuthStore.getState().auth.setUser({ id: 1, username: 'owner', role: 1 })
})
afterEach(() => {
  localStorage.clear()
  useAuthStore.getState().auth.reset()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('one unavailable video does not prevent another video from completing in history', async () => {
  saveVideoWorkspaceTask({ id: 'stale-video', status: 'queued' })
  saveVideoWorkspaceTask({ id: 'active-video', status: 'queued' })
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (String(url).endsWith('/stale-video')) {
      throw new Error('Task unavailable')
    }
    return { data: { id: 'active-video', status: 'completed', progress: 100 } }
  })
  vi.useFakeTimers()
  render(
    <VideoWorkspace
      config={{ ...DEFAULT_CONFIG, model: 'grok-video-1.5' }}
      groups={[]}
      models={[]}
      isModelLoading={false}
      onConfigChange={() => undefined}
    />
  )
  await act(async () => {
    await vi.advanceTimersByTimeAsync(3000)
  })
  expect(
    loadVideoWorkspaceTasks().find((task) => task.taskId === 'active-video')
      ?.status
  ).toBe('completed')
})

test.each([
  { model: 'grok-video-1.5', supportsFourSeconds: false },
  { model: 'grok-imagine-video-1.5', supportsFourSeconds: true },
])(
  '$model offers only durations its channel accepts',
  ({ model, supportsFourSeconds }) => {
    render(
      <VideoWorkspace
        config={{ ...DEFAULT_CONFIG, model }}
        groups={[]}
        models={[]}
        isModelLoading={false}
        onConfigChange={() => undefined}
      />
    )
    const label = screen.getByText('Duration').closest('label')
    if (!label) throw new Error('Duration control is missing')
    fireEvent.click(within(label).getByRole('combobox'))
    expect(screen.queryByRole('option', { name: '4s' }) !== null).toBe(
      supportsFourSeconds
    )
    expect(screen.getByRole('option', { name: '6s' })).toBeInTheDocument()
  }
)
