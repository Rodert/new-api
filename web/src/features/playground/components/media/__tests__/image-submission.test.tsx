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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { DEFAULT_CONFIG } from '../../../constants'
import { saveImageWorkspaceResult } from '../../../lib/storage/storage'
import { ImageWorkspace } from '../image-workspace'

beforeEach(() => {
  localStorage.clear()
  useAuthStore.getState().auth.setUser({ id: 1, username: 'owner', role: 1 })
})
afterEach(() => {
  localStorage.clear()
  useAuthStore.getState().auth.reset()
  vi.restoreAllMocks()
})

test('regenerating a saved image after reload sends every persisted reference URL', async () => {
  saveImageWorkspaceResult(
    { data: [{ url: 'https://example.com/result.png' }] },
    {
      model: 'gpt-image-2',
      group: 'default',
      prompt: 'Edit both references',
      aspectRatio: '1:1',
      qualityPreset: 'auto',
      n: 1,
      size: '1024x1024',
      referenceImages: [
        'https://example.com/one.png',
        'https://example.com/two.png',
      ],
    }
  )
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { id: 'task-new', status: 'queued' } })
  render(
    <ImageWorkspace
      config={{ ...DEFAULT_CONFIG, model: 'gpt-image-2', group: 'default' }}
      groups={[]}
      models={[]}
      isModelLoading={false}
      onConfigChange={() => undefined}
    />
  )
  fireEvent.click(screen.getByRole('button', { name: 'Regenerate' }))
  await waitFor(() =>
    expect(post).toHaveBeenCalledWith(
      '/pg/images/tasks',
      expect.objectContaining({
        model: 'gpt-image-2',
        images: [
          { image_url: 'https://example.com/one.png' },
          { image_url: 'https://example.com/two.png' },
        ],
      }),
      expect.anything()
    )
  )
})

test.each(['generate', 'regenerate'])(
  '%s without references omits images for text-to-image generation',
  async (action) => {
    if (action === 'regenerate') {
      saveImageWorkspaceResult(
        { data: [{ url: 'https://example.com/result.png' }] },
        {
          model: 'gpt-image-2',
          group: 'default',
          prompt: 'Draw a cat',
          aspectRatio: '1:1',
          qualityPreset: 'auto',
          n: 1,
          size: '1024x1024',
          referenceImages: [],
        }
      )
    }
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      data: { id: 'task-text', status: 'queued' },
    })
    render(
      <ImageWorkspace
        config={{ ...DEFAULT_CONFIG, model: 'gpt-image-2', group: 'default' }}
        groups={[]}
        models={[]}
        isModelLoading={false}
        onConfigChange={() => undefined}
      />
    )
    if (action === 'generate') {
      fireEvent.change(screen.getByRole('textbox'), {
        target: { value: 'Draw a cat' },
      })
      fireEvent.click(screen.getByRole('button', { name: 'Create image' }))
    } else {
      fireEvent.click(screen.getByRole('button', { name: 'Regenerate' }))
    }
    await waitFor(() => expect(post).toHaveBeenCalled())
    expect(post.mock.calls[0][1]).toMatchObject({
      model: 'gpt-image-2',
      prompt: 'Draw a cat',
    })
    expect(post.mock.calls[0][1]).not.toHaveProperty('images')
  }
)
