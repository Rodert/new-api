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
// Message types
export type MessageRole = 'user' | 'assistant' | 'system'

export type MessageStatus = 'loading' | 'streaming' | 'complete' | 'error'

export type PlaygroundMessageLayoutMode = 'alternating' | 'left'

export type PlaygroundMode = 'chat' | 'image' | 'video'

export interface MessageVersion {
  id: string
  content: string
}

export interface Message {
  key: string
  from: MessageRole
  versions: MessageVersion[]
  createdAt?: number
  startedAt?: number
  completedAt?: number
  durationMs?: number
  sources?: { href: string; title: string }[]
  reasoning?: {
    content: string
    duration: number
    startedAt?: number
    completedAt?: number
    durationMs?: number
  }
  isReasoningStreaming?: boolean
  isReasoningComplete?: boolean
  isContentComplete?: boolean
  status?: MessageStatus
  errorCode?: string | null
}

// API payload types
export interface ChatCompletionMessage {
  role: MessageRole
  content: string | ContentPart[]
}

export interface ContentPart {
  type: 'text' | 'image_url'
  text?: string
  image_url?: {
    url: string
  }
}

export interface ChatCompletionRequest {
  model: string
  group?: string
  messages: ChatCompletionMessage[]
  stream: boolean
  temperature?: number
  top_p?: number
  max_tokens?: number
  frequency_penalty?: number
  presence_penalty?: number
  seed?: number
}

export interface ChatCompletionChunk {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    delta: {
      role?: MessageRole
      content?: string
      reasoning_content?: string
    }
    finish_reason: string | null
  }>
}

export interface ChatCompletionResponse {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    message: {
      role: MessageRole
      content: string
      reasoning_content?: string
    }
    finish_reason: string
  }>
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}

export interface ImageGenerationRequest {
  model: string
  group?: string
  prompt: string
  n: number
  size: string
  quality?: string
}

export interface ImageGenerationResponse {
  data: Array<{
    url?: string
    b64_json?: string
    revised_prompt?: string
  }>
}

export interface ImageTaskResponse {
  id: string
  object: 'image.task'
  status: 'queued' | 'processing' | 'completed' | 'failed'
  progress?: number
  created_at?: number
  started_at?: number
  completed_at?: number
  data?: ImageGenerationResponse['data']
  error?: {
    message?: string
    type?: string
  }
}

export interface VideoGenerationRequest {
  model: string
  group?: string
  prompt: string
  seconds?: string
  aspect_ratio?: string
  resolution?: string
  image?: string
  images?: string[]
  videos?: string[]
  audios?: string[]
  duration: number
  width: number
  height: number
}

export interface VideoTaskResponse {
  id?: string
  task_id?: string
  object?: string
  model?: string
  status?: string
  progress?: number
  created_at?: number
  completed_at?: number
  failed_at?: number
  processing_time?: number
  seconds?: string
  aspectRatio?: string
  video_url?: string
  result?: {
    duration?: number
    expires_at?: number
    format?: string
    resultUrls?: string[]
    thumbnail_url?: string | null
    video_url?: string
  }
  error?: {
    code?: string
    message?: string
  }
}

// Configuration types
export interface PlaygroundConfig {
  model: string
  group: string
  temperature: number
  top_p: number
  max_tokens: number
  frequency_penalty: number
  presence_penalty: number
  seed: number | null
  stream: boolean
}

export interface ParameterEnabled {
  temperature: boolean
  top_p: boolean
  max_tokens: boolean
  frequency_penalty: boolean
  presence_penalty: boolean
  seed: boolean
}

// Model and group options
export interface ModelOption {
  label: string
  value: string
}

export interface GroupOption {
  label: string
  value: string
  ratio: number
  desc?: string
}
