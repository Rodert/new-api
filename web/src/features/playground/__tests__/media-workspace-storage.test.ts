import assert from 'node:assert/strict'
import { afterEach, beforeEach, describe, test } from 'node:test'

import { STORAGE_KEYS } from '../constants'
import {
  clearPlaygroundData,
  loadImageWorkspaceResult,
  loadVideoWorkspaceTask,
  saveImageWorkspaceResult,
  saveVideoWorkspaceTask,
} from '../lib/storage/storage'

const originalLocalStorage = Object.getOwnPropertyDescriptor(
  globalThis,
  'localStorage'
)

function createLocalStorage(): Storage {
  const values = new Map<string, string>()

  return {
    get length() {
      return values.size
    },
    clear: () => values.clear(),
    getItem: (key) => values.get(key) ?? null,
    key: (index) => [...values.keys()][index] ?? null,
    removeItem: (key) => values.delete(key),
    setItem: (key, value) => values.set(key, value),
  }
}

beforeEach(() => {
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: createLocalStorage(),
  })
})

afterEach(() => {
  if (originalLocalStorage) {
    Object.defineProperty(globalThis, 'localStorage', originalLocalStorage)
    return
  }

  Reflect.deleteProperty(globalThis, 'localStorage')
})

describe('media workspace storage', () => {
  test('keeps image and video results in the same workspace', () => {
    const imageResult = {
      data: [{ url: 'https://example.com/generated-image.png' }],
    }
    const videoTask = {
      task_id: 'task_123',
      status: 'queued',
    }

    saveImageWorkspaceResult(imageResult)
    saveVideoWorkspaceTask(videoTask)

    assert.deepEqual(loadImageWorkspaceResult(), imageResult)
    assert.equal(loadVideoWorkspaceTask()?.task_id, videoTask.task_id)
    assert.equal(loadVideoWorkspaceTask()?.status, videoTask.status)

    const workspace = JSON.parse(
      localStorage.getItem(STORAGE_KEYS.WORKSPACE) ?? '{}'
    ) as {
      version: number
      mode: string
      chat: Record<string, unknown>
      image: { config: Record<string, unknown>; items: unknown[] }
      video: { config: Record<string, unknown>; tasks: unknown[] }
      data?: unknown
    }

    assert.equal(workspace.version, 1)
    assert.equal(workspace.data, undefined)
    assert.equal(workspace.mode, 'chat')
    assert.ok(workspace.chat)
    assert.equal(workspace.image.config.response_format, 'url')
    assert.equal(workspace.image.items.length, 1)
    assert.equal(workspace.video.config.n, 1)
    assert.equal(workspace.video.tasks.length, 1)
  })

  test('removes workspace data when playground data is cleared', () => {
    saveVideoWorkspaceTask({ task_id: 'task_123', status: 'queued' })

    clearPlaygroundData()

    assert.equal(localStorage.getItem(STORAGE_KEYS.WORKSPACE), null)
  })
})
