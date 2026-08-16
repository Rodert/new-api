import assert from 'node:assert/strict'
import { afterEach, beforeEach, describe, test } from 'node:test'

import { STORAGE_KEYS } from '../constants'
import {
  clearPlaygroundData,
  clearImageWorkspaceItems,
  clearVideoWorkspaceTasks,
  deleteImageWorkspaceItem,
  deleteVideoWorkspaceTask,
  loadImageWorkspaceItems,
  loadImageWorkspaceResult,
  loadVideoWorkspaceTasks,
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
    assert.equal('n' in workspace.video.config, false)
    assert.equal(workspace.video.tasks.length, 1)
  })

  test('removes workspace data when playground data is cleared', () => {
    saveVideoWorkspaceTask({ task_id: 'task_123', status: 'queued' })

    clearPlaygroundData()

    assert.equal(localStorage.getItem(STORAGE_KEYS.WORKSPACE), null)
  })

  test('keeps multiple image records and removes only the selected record', () => {
    saveImageWorkspaceResult(
      { data: [{ url: 'https://example.com/first.png' }] },
      {
        aspectRatio: '1:1',
        group: 'default',
        model: 'gpt-image-2',
        n: 1,
        prompt: 'first image',
        qualityPreset: 'hd',
        size: '1024x1024',
      }
    )
    saveImageWorkspaceResult(
      { data: [{ url: 'https://example.com/second.png' }] },
      {
        aspectRatio: '16:9',
        group: 'default',
        model: 'gpt-image-2',
        n: 2,
        prompt: 'second image',
        qualityPreset: 'auto',
        size: '1792x1024',
      }
    )

    const items = loadImageWorkspaceItems()
    assert.equal(items.length, 2)
    assert.equal(items[0]?.prompt, 'second image')
    assert.equal(items[0]?.size, '1792x1024')

    deleteImageWorkspaceItem(items[0]?.id ?? '')

    assert.equal(loadImageWorkspaceItems().length, 1)
    assert.equal(loadImageWorkspaceItems()[0]?.prompt, 'first image')

    clearImageWorkspaceItems()
    assert.deepEqual(loadImageWorkspaceItems(), [])
  })

  test('keeps multiple video tasks and preserves their generation settings', () => {
    saveVideoWorkspaceTask(
      { task_id: 'task_first', status: 'queued' },
      {
        aspectRatio: '16:9',
        group: 'default',
        model: 'as-sd2.0-fast',
        n: 1,
        prompt: 'first video',
        resolution: '720p',
        referenceImages: ['https://example.com/reference.png'],
        seconds: '15',
      }
    )
    saveVideoWorkspaceTask(
      { task_id: 'task_second', status: 'processing' },
      {
        aspectRatio: '9:16',
        group: 'default',
        model: 'as-sd2.0-fast',
        n: 2,
        prompt: 'second video',
        resolution: '1080p',
        referenceAudio: ['https://example.com/music.mp3'],
        seconds: '10',
      }
    )

    const tasks = loadVideoWorkspaceTasks()
    assert.equal(tasks.length, 2)
    assert.equal(tasks[0]?.taskId, 'task_second')
    assert.equal(tasks[0]?.resolution, '1080p')
    assert.deepEqual(tasks[0]?.referenceAudio, [
      'https://example.com/music.mp3',
    ])

    deleteVideoWorkspaceTask('task_second')

    assert.equal(loadVideoWorkspaceTasks().length, 1)
    assert.equal(loadVideoWorkspaceTasks()[0]?.taskId, 'task_first')

    clearVideoWorkspaceTasks()
    assert.deepEqual(loadVideoWorkspaceTasks(), [])
  })
})
