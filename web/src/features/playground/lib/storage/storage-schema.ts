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
import { z } from 'zod'

export const STORAGE_VERSION = 1
export const MAX_STORED_MESSAGES = 100
export const MAX_STORED_MESSAGES_BYTES = 1024 * 1024
export const MAX_LOADED_MESSAGES_CHARS = 120_000
export const MAX_LOADED_MESSAGE_CHARS = 40_000

export const playgroundConfigSchema = z.object({
  model: z.string().optional(),
  group: z.string().optional(),
  temperature: z.number().optional(),
  top_p: z.number().optional(),
  max_tokens: z.number().optional(),
  frequency_penalty: z.number().optional(),
  presence_penalty: z.number().optional(),
  seed: z.number().nullable().optional(),
  stream: z.boolean().optional(),
})

export const parameterEnabledSchema = z.object({
  temperature: z.boolean().optional(),
  top_p: z.boolean().optional(),
  max_tokens: z.boolean().optional(),
  frequency_penalty: z.boolean().optional(),
  presence_penalty: z.boolean().optional(),
  seed: z.boolean().optional(),
})

const messageRoleSchema = z.enum(['user', 'assistant', 'system'])
const messageStatusSchema = z.enum([
  'loading',
  'streaming',
  'complete',
  'error',
])

const messageVersionSchema = z.object({
  id: z.string(),
  content: z.string(),
})

const sourceSchema = z.object({
  href: z.string(),
  title: z.string(),
})

const reasoningSchema = z.object({
  content: z.string(),
  duration: z.number(),
  startedAt: z.number().optional(),
  completedAt: z.number().optional(),
  durationMs: z.number().optional(),
})

const messageSchema = z.object({
  key: z.string(),
  from: messageRoleSchema,
  versions: z.array(messageVersionSchema).min(1),
  createdAt: z.number().optional(),
  startedAt: z.number().optional(),
  completedAt: z.number().optional(),
  durationMs: z.number().optional(),
  sources: z.array(sourceSchema).optional(),
  reasoning: reasoningSchema.optional(),
  isReasoningStreaming: z.boolean().optional(),
  isReasoningComplete: z.boolean().optional(),
  isContentComplete: z.boolean().optional(),
  status: messageStatusSchema.optional(),
  errorCode: z.string().nullable().optional(),
})

export const messagesSchema = z.array(messageSchema)

const imageResultSchema = z.object({
  url: z.string().optional(),
  b64_json: z.string().optional(),
  revised_prompt: z.string().optional(),
})

const imageWorkspaceConfigSchema = z.object({
  model: z.string(),
  group: z.string(),
  aspectRatio: z.string(),
  qualityPreset: z.string(),
  n: z.number(),
  response_format: z.literal('url'),
})

const imageWorkspaceItemSchema = z.object({
  id: z.string(),
  prompt: z.string(),
  model: z.string(),
  group: z.string(),
  aspectRatio: z.string(),
  qualityPreset: z.string(),
  n: z.number(),
  createdAt: z.number(),
  status: z.enum(['loading', 'completed', 'error']),
  error: z.string().optional(),
  size: z.string().optional(),
  referenceImages: z.array(z.string()).optional(),
  data: z.array(imageResultSchema),
})

const videoTaskResponseSchema = z
  .object({
    id: z.string().optional(),
    task_id: z.string().optional(),
    object: z.string().optional(),
    model: z.string().optional(),
    status: z.string().optional(),
    progress: z.number().optional(),
    created_at: z.number().optional(),
    completed_at: z.number().optional(),
    failed_at: z.number().optional(),
    processing_time: z.number().optional(),
    seconds: z.string().optional(),
    aspectRatio: z.string().optional(),
    video_url: z.string().optional(),
    result: z
      .object({
        duration: z.number().optional(),
        expires_at: z.number().optional(),
        format: z.string().optional(),
        resultUrls: z.array(z.string()).optional(),
        thumbnail_url: z.string().nullable().optional(),
        video_url: z.string().optional(),
      })
      .optional(),
    error: z
      .object({
        code: z.string().optional(),
        message: z.string().optional(),
      })
      .optional(),
  })
  .passthrough()

const videoWorkspaceConfigSchema = z.object({
  model: z.string(),
  group: z.string(),
  aspectRatio: z.string(),
  seconds: z.string(),
  qualityPreset: z.string(),
  n: z.number().int().min(1).max(10),
})

const videoWorkspaceTaskSchema = videoTaskResponseSchema.extend({
  id: z.string(),
  taskId: z.string(),
  model: z.string(),
  group: z.string(),
  prompt: z.string(),
  aspectRatio: z.string(),
  seconds: z.string(),
  qualityPreset: z.string(),
  n: z.number().int().min(1).max(10).optional(),
  referenceImages: z.array(z.string()).optional(),
  referenceVideos: z.array(z.string()).optional(),
  referenceAudio: z.array(z.string()).optional(),
  createdAt: z.number(),
})

export const workspaceSchema = z.object({
  version: z.literal(1),
  mode: z.enum(['chat', 'image', 'video']),
  chat: z.object({
    config: playgroundConfigSchema,
    parameterEnabled: parameterEnabledSchema,
    messages: messagesSchema,
  }),
  image: z.object({
    config: imageWorkspaceConfigSchema,
    items: z.array(imageWorkspaceItemSchema).max(24),
  }),
  video: z.object({
    config: videoWorkspaceConfigSchema,
    tasks: z.array(videoWorkspaceTaskSchema).max(24),
  }),
})
