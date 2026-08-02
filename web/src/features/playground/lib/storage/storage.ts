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
import {
  DEFAULT_CONFIG,
  DEFAULT_PARAMETER_ENABLED,
  MESSAGE_STATUS,
  STORAGE_KEYS,
} from '../../constants'
import type {
  ImageGenerationResponse,
  Message,
  ParameterEnabled,
  PlaygroundConfig,
  PlaygroundMode,
  VideoTaskResponse,
} from '../../types'
import {
  finalizeMessage,
  isAssistantMessagePending,
  sanitizeMessagesOnLoad,
} from '../message/message-streaming-utils'
import { completeAssistantTiming } from '../message/message-timing-utils'
import { hasMessageContent } from '../message/message-utils'
import {
  MAX_LOADED_MESSAGE_CHARS,
  MAX_LOADED_MESSAGES_CHARS,
  MAX_STORED_MESSAGES,
  MAX_STORED_MESSAGES_BYTES,
  STORAGE_VERSION,
  messagesSchema,
  parameterEnabledSchema,
  playgroundConfigSchema,
  workspaceSchema,
} from './storage-schema'

type StoredEnvelope<T> = {
  version: number
  data: T
}

type ImageWorkspaceConfig = {
  model: string
  group: string
  aspectRatio: string
  qualityPreset: string
  n: number
  response_format: 'url'
}

type ImageWorkspaceItem = {
  id: string
  prompt: string
  model: string
  group: string
  aspectRatio: string
  qualityPreset: string
  n: number
  createdAt: number
  status: 'loading' | 'completed' | 'error'
  error?: string
  data: ImageGenerationResponse['data']
}

type VideoWorkspaceConfig = {
  model: string
  group: string
  aspectRatio: string
  seconds: string
  qualityPreset: string
  n: 1
}

type VideoWorkspaceTask = VideoTaskResponse & {
  id: string
  taskId: string
  model: string
  group: string
  prompt: string
  aspectRatio: string
  seconds: string
  qualityPreset: string
  createdAt: number
}

type PlaygroundWorkspace = {
  version: 1
  mode: PlaygroundMode
  chat: {
    config: PlaygroundConfig
    parameterEnabled: ParameterEnabled
    messages: Message[]
  }
  image: {
    config: ImageWorkspaceConfig
    items: ImageWorkspaceItem[]
  }
  video: {
    config: VideoWorkspaceConfig
    tasks: VideoWorkspaceTask[]
  }
}

const TRUNCATED_CONTENT_SUFFIX = '\n\n[...]'
const MIN_PREFIX_COLLAPSE_LENGTH = 2000
const MIN_REPEATED_SECTION_COUNT = 3
const SECTION_HEADING_LINE_PATTERN = /^#{2,6}\s+\d+\.\s+.+$/gm

function readStoredValue(key: string): unknown | null {
  const saved = localStorage.getItem(key)
  if (!saved) return null

  return JSON.parse(saved) as unknown
}

function readStoredMessagesValue(): unknown | null {
  const saved = localStorage.getItem(STORAGE_KEYS.MESSAGES)
  if (!saved) return null

  if (saved.length > MAX_STORED_MESSAGES_BYTES) {
    localStorage.removeItem(STORAGE_KEYS.MESSAGES)
    return null
  }

  return JSON.parse(saved) as unknown
}

function unwrapStoredValue(value: unknown): unknown {
  if (!value || typeof value !== 'object') {
    return value
  }

  if ('version' in value && 'data' in value) {
    return (value as StoredEnvelope<unknown>).data
  }

  return value
}

function createWorkspace(): PlaygroundWorkspace {
  return {
    version: STORAGE_VERSION,
    mode: 'chat',
    chat: {
      config: { ...DEFAULT_CONFIG },
      parameterEnabled: { ...DEFAULT_PARAMETER_ENABLED },
      messages: [],
    },
    image: {
      config: {
        model: 'gpt-image-1',
        group: DEFAULT_CONFIG.group,
        aspectRatio: '1:1',
        qualityPreset: 'auto',
        n: 1,
        response_format: 'url',
      },
      items: [],
    },
    video: {
      config: {
        model: 'video-ds-2.0-fast',
        group: DEFAULT_CONFIG.group,
        aspectRatio: '16:9',
        seconds: '5',
        qualityPreset: 'recommended',
        n: 1,
      },
      tasks: [],
    },
  }
}

function createWorkspaceItemID(): string {
  return crypto.randomUUID()
}

function migrateLegacyWorkspace(value: unknown): PlaygroundWorkspace {
  const workspace = createWorkspace()
  if (!value || typeof value !== 'object') {
    return workspace
  }

  const legacy = value as {
    imageResult?: ImageGenerationResponse
    videoTask?: VideoTaskResponse
  }
  const savedConfig = readStoredValue(STORAGE_KEYS.CONFIG)
  const parsedConfig = savedConfig
    ? playgroundConfigSchema.safeParse(unwrapStoredValue(savedConfig))
    : null
  if (parsedConfig?.success) {
    workspace.chat.config = { ...workspace.chat.config, ...parsedConfig.data }
  }

  const savedParameterEnabled = readStoredValue(STORAGE_KEYS.PARAMETER_ENABLED)
  const parsedParameterEnabled = savedParameterEnabled
    ? parameterEnabledSchema.safeParse(unwrapStoredValue(savedParameterEnabled))
    : null
  if (parsedParameterEnabled?.success) {
    workspace.chat.parameterEnabled = {
      ...workspace.chat.parameterEnabled,
      ...parsedParameterEnabled.data,
    }
  }

  const savedMessages = readStoredMessagesValue()
  const parsedMessages = savedMessages
    ? messagesSchema.safeParse(unwrapStoredValue(savedMessages))
    : null
  if (parsedMessages?.success) {
    workspace.chat.messages = parsedMessages.data
  }

  if (legacy.imageResult?.data) {
    workspace.image.items = [
      {
        id: createWorkspaceItemID(),
        prompt: '',
        model: workspace.image.config.model,
        group: workspace.image.config.group,
        aspectRatio: workspace.image.config.aspectRatio,
        qualityPreset: workspace.image.config.qualityPreset,
        n: workspace.image.config.n,
        createdAt: Date.now(),
        status: 'completed',
        data: legacy.imageResult.data,
      },
    ]
  }

  const taskID = legacy.videoTask?.task_id ?? legacy.videoTask?.id
  if (legacy.videoTask && taskID) {
    workspace.video.tasks = [
      {
        ...legacy.videoTask,
        id: createWorkspaceItemID(),
        taskId: taskID,
        model: workspace.video.config.model,
        group: workspace.video.config.group,
        prompt: '',
        aspectRatio: workspace.video.config.aspectRatio,
        seconds: workspace.video.config.seconds,
        qualityPreset: workspace.video.config.qualityPreset,
        createdAt: Date.now(),
      },
    ]
  }

  return workspace
}

function loadWorkspace(): PlaygroundWorkspace {
  const saved = readStoredValue(STORAGE_KEYS.WORKSPACE)
  if (!saved) return createWorkspace()

  const value = unwrapStoredValue(saved)
  const result = workspaceSchema.safeParse(value)
  if (!result.success) {
    return migrateLegacyWorkspace(value)
  }

  const parsed = result.data
  const defaults = createWorkspace()

  return {
    ...defaults,
    ...parsed,
    chat: {
      ...defaults.chat,
      ...parsed.chat,
      config: { ...defaults.chat.config, ...parsed.chat.config },
      parameterEnabled: {
        ...defaults.chat.parameterEnabled,
        ...parsed.chat.parameterEnabled,
      },
    },
    image: {
      ...defaults.image,
      ...parsed.image,
      config: { ...defaults.image.config, ...parsed.image.config },
    },
    video: {
      ...defaults.video,
      ...parsed.video,
      config: { ...defaults.video.config, ...parsed.video.config },
    },
  }
}

function writeWorkspace(workspace: PlaygroundWorkspace): void {
  const persistedWorkspace: PlaygroundWorkspace = {
    ...workspace,
    chat: {
      ...workspace.chat,
      messages: trimMessages(workspace.chat.messages),
    },
  }

  localStorage.setItem(
    STORAGE_KEYS.WORKSPACE,
    JSON.stringify(persistedWorkspace)
  )
}

function updateWorkspace(
  update: (workspace: PlaygroundWorkspace) => PlaygroundWorkspace
): void {
  writeWorkspace(update(loadWorkspace()))
}

function trimMessages(messages: Message[]): Message[] {
  if (messages.length <= MAX_STORED_MESSAGES) {
    return messages
  }

  return messages.slice(-MAX_STORED_MESSAGES)
}

function getMessageSize(message: Message): number {
  const versionsSize = message.versions.reduce(
    (total, version) => total + version.content.length,
    0
  )
  const reasoningSize = message.reasoning?.content.length ?? 0

  return versionsSize + reasoningSize
}

function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) {
    return text
  }

  if (maxLength <= TRUNCATED_CONTENT_SUFFIX.length) {
    return text.slice(0, maxLength)
  }

  return `${text.slice(0, maxLength - TRUNCATED_CONTENT_SUFFIX.length)}${TRUNCATED_CONTENT_SUFFIX}`
}

type SectionOccurrence = {
  heading: string
  index: number
}

function getSectionOccurrences(text: string): SectionOccurrence[] {
  const occurrences: SectionOccurrence[] = []
  const matches = text.matchAll(SECTION_HEADING_LINE_PATTERN)
  for (const match of matches) {
    const index = match.index
    if (index === undefined) {
      continue
    }

    occurrences.push({
      heading: match[0],
      index,
    })
  }

  return occurrences
}

function getHeadingCounts(
  occurrences: SectionOccurrence[]
): Map<string, number> {
  const counts = new Map<string, number>()

  for (const occurrence of occurrences) {
    counts.set(occurrence.heading, (counts.get(occurrence.heading) ?? 0) + 1)
  }

  return counts
}

function findLastRepeatedSectionRunStart(text: string): number {
  const occurrences = getSectionOccurrences(text)
  const headingCounts = getHeadingCounts(occurrences)
  const lastRepeatedIndexes: number[] = []
  const seenHeadings = new Set<string>()

  for (let index = occurrences.length - 1; index >= 0; index--) {
    const occurrence = occurrences[index]
    const count = headingCounts.get(occurrence.heading) ?? 0

    if (
      count < MIN_REPEATED_SECTION_COUNT ||
      seenHeadings.has(occurrence.heading)
    ) {
      continue
    }

    seenHeadings.add(occurrence.heading)
    lastRepeatedIndexes.push(occurrence.index)
  }

  if (lastRepeatedIndexes.length === 0) {
    return -1
  }

  return Math.min(...lastRepeatedIndexes)
}

function collapseRepeatedSectionSnapshots(text: string): string {
  if (text.length < MIN_PREFIX_COLLAPSE_LENGTH) {
    return text
  }

  const lastRepeatedRunStart = findLastRepeatedSectionRunStart(text)
  if (lastRepeatedRunStart === -1) {
    return text
  }

  return text.slice(lastRepeatedRunStart)
}

function normalizeStoredMessageForLoad(message: Message): Message {
  let changed = false
  const versions = message.versions.map((version) => {
    const collapsedContent = collapseRepeatedSectionSnapshots(version.content)
    const content = truncateText(collapsedContent, MAX_LOADED_MESSAGE_CHARS)

    if (content === version.content && collapsedContent === version.content) {
      return version
    }

    changed = true
    return {
      ...version,
      content,
    }
  })

  const reasoning = message.reasoning
    ? {
        ...message.reasoning,
        content: truncateText(
          message.reasoning.content,
          MAX_LOADED_MESSAGE_CHARS
        ),
      }
    : undefined

  if (reasoning?.content !== message.reasoning?.content) {
    changed = true
  }

  const normalized = changed ? { ...message, versions, reasoning } : message

  if (!isAssistantMessagePending(normalized)) {
    return normalized
  }

  const hasContent = hasMessageContent(normalized)
  const hasReasoning = normalized.reasoning?.content.trim()

  if (!hasContent && !hasReasoning) {
    return normalized
  }

  const completedAt =
    normalized.completedAt ??
    normalized.reasoning?.completedAt ??
    normalized.startedAt ??
    normalized.createdAt ??
    Date.now()

  return completeAssistantTiming(
    {
      ...finalizeMessage(normalized),
      status: MESSAGE_STATUS.COMPLETE,
      isReasoningStreaming: false,
    },
    completedAt
  )
}

function trimMessagesByContentSize(messages: Message[]): Message[] {
  let totalSize = 0
  const result: Message[] = []

  for (let index = messages.length - 1; index >= 0; index--) {
    const message = messages[index]
    const messageSize = getMessageSize(message)

    if (
      result.length > 0 &&
      totalSize + messageSize > MAX_LOADED_MESSAGES_CHARS
    ) {
      break
    }

    totalSize += messageSize
    result.push(message)
  }

  return result.reverse()
}

/**
 * Load playground config from localStorage
 */
export function loadConfig(): Partial<PlaygroundConfig> {
  try {
    const workspace = readStoredValue(STORAGE_KEYS.WORKSPACE)
    if (workspace) {
      return loadWorkspace().chat.config
    }

    const saved = readStoredValue(STORAGE_KEYS.CONFIG)
    return saved ? playgroundConfigSchema.parse(unwrapStoredValue(saved)) : {}
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load config:', error)
  }
  return {}
}

/**
 * Save playground config to localStorage
 */
export function saveConfig(config: Partial<PlaygroundConfig>): void {
  try {
    const parsed = playgroundConfigSchema.parse(config)
    updateWorkspace((workspace) => ({
      ...workspace,
      chat: {
        ...workspace.chat,
        config: { ...workspace.chat.config, ...parsed },
      },
    }))
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save config:', error)
  }
}

/**
 * Load parameter enabled state from localStorage
 */
export function loadParameterEnabled(): Partial<ParameterEnabled> {
  try {
    const workspace = readStoredValue(STORAGE_KEYS.WORKSPACE)
    if (workspace) {
      return loadWorkspace().chat.parameterEnabled
    }

    const saved = readStoredValue(STORAGE_KEYS.PARAMETER_ENABLED)
    return saved ? parameterEnabledSchema.parse(unwrapStoredValue(saved)) : {}
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load parameter enabled:', error)
  }
  return {}
}

/**
 * Save parameter enabled state to localStorage
 */
export function saveParameterEnabled(
  parameterEnabled: Partial<ParameterEnabled>
): void {
  try {
    const parsed = parameterEnabledSchema.parse(parameterEnabled)
    updateWorkspace((workspace) => ({
      ...workspace,
      chat: {
        ...workspace.chat,
        parameterEnabled: { ...workspace.chat.parameterEnabled, ...parsed },
      },
    }))
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save parameter enabled:', error)
  }
}

/**
 * Load messages from localStorage
 */
export function loadMessages(): Message[] | null {
  try {
    const workspace = readStoredValue(STORAGE_KEYS.WORKSPACE)
    const saved = workspace
      ? loadWorkspace().chat.messages
      : readStoredMessagesValue()
    if (!saved) return null

    const parsed = messagesSchema.parse(unwrapStoredValue(saved)) as Message[]
    const normalized = parsed.map(normalizeStoredMessageForLoad)
    const normalizedChanged = normalized.some(
      (message, index) => message !== parsed[index]
    )
    const trimmed = trimMessages(normalized)
    const sizeTrimmed = trimMessagesByContentSize(trimmed)
    const sanitized = sanitizeMessagesOnLoad(sizeTrimmed)

    if (
      normalizedChanged ||
      trimmed !== normalized ||
      sizeTrimmed !== trimmed ||
      sanitized !== sizeTrimmed
    ) {
      saveMessages(sanitized)
    }

    return sanitized
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load messages:', error)
  }
  return null
}

/**
 * Save messages to localStorage
 */
export function saveMessages(messages: Message[]): void {
  try {
    const trimmed = trimMessages(messages)
    const parsed = messagesSchema.parse(trimmed) as Message[]
    updateWorkspace((workspace) => ({
      ...workspace,
      chat: {
        ...workspace.chat,
        messages: parsed,
      },
    }))
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save messages:', error)
  }
}

export function loadImageWorkspaceResult(): ImageGenerationResponse | null {
  try {
    const item = loadWorkspace().image.items[0]
    return item ? { data: item.data } : null
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load image workspace:', error)
  }
  return null
}

export function saveImageWorkspaceResult(
  imageResult: ImageGenerationResponse | null,
  metadata?: {
    aspectRatio: string
    group: string
    model: string
    n: number
    prompt: string
    qualityPreset: string
  }
): void {
  try {
    updateWorkspace((workspace) => {
      if (!imageResult) {
        return {
          ...workspace,
          image: { ...workspace.image, items: [] },
        }
      }

      const imageConfig: ImageWorkspaceConfig = metadata
        ? {
            model: metadata.model,
            group: metadata.group,
            aspectRatio: metadata.aspectRatio,
            qualityPreset: metadata.qualityPreset,
            n: metadata.n,
            response_format: 'url',
          }
        : workspace.image.config
      const item: ImageWorkspaceItem = {
        id: createWorkspaceItemID(),
        prompt: metadata?.prompt ?? '',
        model: imageConfig.model,
        group: imageConfig.group,
        aspectRatio: imageConfig.aspectRatio,
        qualityPreset: imageConfig.qualityPreset,
        n: imageConfig.n,
        createdAt: Date.now(),
        status: 'completed',
        data: imageResult.data,
      }

      return {
        ...workspace,
        image: {
          config: imageConfig,
          items: [item, ...workspace.image.items].slice(0, 24),
        },
      }
    })
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save image workspace:', error)
  }
}

export function loadVideoWorkspaceTask(): VideoTaskResponse | null {
  try {
    const task = loadWorkspace().video.tasks[0]
    if (!task) return null

    const { taskId, ...videoTask } = task
    return { ...videoTask, task_id: taskId }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load video workspace:', error)
  }
  return null
}

export function saveVideoWorkspaceTask(
  videoTask: VideoTaskResponse | null,
  metadata?: {
    aspectRatio: string
    group: string
    model: string
    prompt: string
    qualityPreset: string
    seconds: string
  }
): void {
  try {
    updateWorkspace((workspace) => {
      if (!videoTask) {
        return {
          ...workspace,
          video: { ...workspace.video, tasks: [] },
        }
      }

      const taskId = videoTask.task_id ?? videoTask.id
      if (!taskId) {
        throw new Error('Video task ID is required for workspace storage')
      }

      const videoConfig: VideoWorkspaceConfig = metadata
        ? {
            model: metadata.model,
            group: metadata.group,
            aspectRatio: metadata.aspectRatio,
            seconds: metadata.seconds,
            qualityPreset: metadata.qualityPreset,
            n: 1,
          }
        : workspace.video.config
      const existingTask = workspace.video.tasks.find(
        (task) => task.taskId === taskId
      )
      const item: VideoWorkspaceTask = {
        ...existingTask,
        ...videoTask,
        id: existingTask?.id ?? createWorkspaceItemID(),
        taskId,
        model: metadata?.model ?? existingTask?.model ?? videoConfig.model,
        group: metadata?.group ?? existingTask?.group ?? videoConfig.group,
        prompt: metadata?.prompt ?? existingTask?.prompt ?? '',
        aspectRatio:
          metadata?.aspectRatio ??
          existingTask?.aspectRatio ??
          videoConfig.aspectRatio,
        seconds:
          metadata?.seconds ?? existingTask?.seconds ?? videoConfig.seconds,
        qualityPreset:
          metadata?.qualityPreset ??
          existingTask?.qualityPreset ??
          videoConfig.qualityPreset,
        createdAt: existingTask?.createdAt ?? Date.now(),
      }

      return {
        ...workspace,
        video: {
          config: videoConfig,
          tasks: existingTask
            ? workspace.video.tasks.map((task) =>
                task.taskId === taskId ? item : task
              )
            : [item, ...workspace.video.tasks].slice(0, 24),
        },
      }
    })
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save video workspace:', error)
  }
}

export function loadPlaygroundMode(): PlaygroundMode {
  try {
    const saved = readStoredValue(STORAGE_KEYS.WORKSPACE)
    return saved ? loadWorkspace().mode : 'chat'
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load playground mode:', error)
  }
  return 'chat'
}

export function savePlaygroundMode(mode: PlaygroundMode): void {
  try {
    updateWorkspace((workspace) => ({ ...workspace, mode }))
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save playground mode:', error)
  }
}

/**
 * Clear all playground data
 */
export function clearPlaygroundData(): void {
  try {
    localStorage.removeItem(STORAGE_KEYS.CONFIG)
    localStorage.removeItem(STORAGE_KEYS.PARAMETER_ENABLED)
    localStorage.removeItem(STORAGE_KEYS.MESSAGES)
    localStorage.removeItem(STORAGE_KEYS.WORKSPACE)
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to clear playground data:', error)
  }
}
