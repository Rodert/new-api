/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { ImageIcon, MessageSquareIcon, VideoIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type {
  GroupOption,
  Message,
  ModelOption,
  ParameterEnabled,
  PlaygroundConfig,
  PlaygroundMode,
} from '../../types'
import { PlaygroundChat } from '../chat/playground-chat'
import { PlaygroundInput } from '../input/playground-input'
import { ImageWorkspace } from './image-workspace'
import { VideoWorkspace } from './video-workspace'

type MediaWorkspaceProps = {
  config: PlaygroundConfig
  groups: GroupOption[]
  isGenerating: boolean
  isLoadingMessages: boolean
  isModelLoading: boolean
  messages: Message[]
  mode: PlaygroundMode
  models: ModelOption[]
  parameterEnabled: ParameterEnabled
  onClearMessages: () => void
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
  onModeChange: (mode: PlaygroundMode) => void
  onDeleteMessage: (message: Message) => void
  onEditMessage: (message: Message) => void
  onEditOpenChange: (open: boolean) => void
  onParameterEnabledChange: (
    key: keyof ParameterEnabled,
    value: boolean
  ) => void
  onRegenerateMessage: (message: Message) => void
  onSaveEdit: (content: string, submit: boolean) => void
  onSendMessage: (content: string) => void
  onStop: () => void
  editingMessageKey: string | null
}

export function MediaWorkspace(props: MediaWorkspaceProps) {
  const { t } = useTranslation()

  return (
    <Tabs
      className='size-full'
      onValueChange={(value) => props.onModeChange(value as PlaygroundMode)}
      value={props.mode}
    >
      <div className='border-border/70 bg-background/80 flex h-12 shrink-0 items-center border-b px-3 backdrop-blur md:px-5'>
        <TabsList className='h-8'>
          <TabsTrigger value='chat'>
            <MessageSquareIcon />
            {t('Chat')}
          </TabsTrigger>
          <TabsTrigger value='image'>
            <ImageIcon />
            {t('Image')}
          </TabsTrigger>
          <TabsTrigger value='video'>
            <VideoIcon />
            {t('Video')}
          </TabsTrigger>
        </TabsList>
      </div>

      <TabsContent className='flex min-h-0' value='chat'>
        <div className='flex size-full min-h-0 flex-col overflow-hidden'>
          <PlaygroundChat
            editingKey={props.editingMessageKey}
            isGenerating={props.isGenerating}
            isLoadingMessages={props.isLoadingMessages}
            messages={props.messages}
            onCancelEdit={props.onEditOpenChange}
            onDeleteMessage={props.onDeleteMessage}
            onEditMessage={props.onEditMessage}
            onRegenerateMessage={props.onRegenerateMessage}
            onSaveEdit={(content) => props.onSaveEdit(content, false)}
            onSaveEditAndSubmit={(content) => props.onSaveEdit(content, true)}
            onSelectPrompt={props.onSendMessage}
          />
          <div className='mx-auto w-full max-w-4xl px-3 pb-3 md:px-4 md:pb-4'>
            <PlaygroundInput
              config={props.config}
              disabled={props.isGenerating}
              groupValue={props.config.group}
              groups={props.groups}
              hasMessages={props.messages.length > 0}
              isGenerating={props.isGenerating}
              isModelLoading={props.isModelLoading}
              modelValue={props.config.model}
              models={props.models}
              onClearMessages={props.onClearMessages}
              onConfigChange={props.onConfigChange}
              onGroupChange={(value) => props.onConfigChange('group', value)}
              onModelChange={(value) => props.onConfigChange('model', value)}
              onParameterEnabledChange={props.onParameterEnabledChange}
              onStop={props.onStop}
              onSubmit={props.onSendMessage}
              parameterEnabled={props.parameterEnabled}
            />
          </div>
        </div>
      </TabsContent>

      <TabsContent className='flex min-h-0' value='image'>
        <ImageWorkspace
          config={props.config}
          groups={props.groups}
          isModelLoading={props.isModelLoading}
          models={props.models}
          onConfigChange={props.onConfigChange}
        />
      </TabsContent>

      <TabsContent className='flex min-h-0' value='video'>
        <VideoWorkspace
          config={props.config}
          groups={props.groups}
          isModelLoading={props.isModelLoading}
          models={props.models}
          onConfigChange={props.onConfigChange}
        />
      </TabsContent>
    </Tabs>
  )
}
