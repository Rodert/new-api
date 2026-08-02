import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getVideoURL } from '../lib/video-task'

describe('getVideoURL', () => {
  test('uses the same-origin content endpoint for a completed task', () => {
    const task = {
      id: 'task_public_id',
      status: 'completed',
      result: {
        video_url: 'https://cdn.example/result.mp4',
      },
      video_url: 'https://cdn.example/video.mp4',
    }

    assert.equal(getVideoURL(task), '/pg/videos/task_public_id/content')
  })

  test('does not expose a content URL before the task completes', () => {
    const task = {
      id: 'task_public_id',
      status: 'processing',
    }

    assert.equal(getVideoURL(task), null)
  })
})
