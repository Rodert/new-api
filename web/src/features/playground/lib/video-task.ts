import type { VideoTaskResponse } from '../types'

const playableStatuses = new Set(['completed', 'succeeded', 'success'])

export function getVideoURL(task: VideoTaskResponse | null): string | null {
  const taskID = task?.task_id ?? task?.id
  const status = task?.status?.toLowerCase()
  if (!taskID || !status || !playableStatuses.has(status)) {
    return null
  }

  return `/pg/videos/${encodeURIComponent(taskID)}/content`
}
