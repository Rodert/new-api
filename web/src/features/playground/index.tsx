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
import { MediaWorkspace } from './components/media/media-workspace'
import {
  useChatHandler,
  usePlaygroundConversation,
  usePlaygroundOptions,
  usePlaygroundState,
} from './hooks'

export function Playground() {
  const {
    config,
    parameterEnabled,
    messages,
    isLoadingMessages,
    models,
    groups,
    updateMessages,
    setModels,
    setGroups,
    updateConfig,
    updateParameterEnabled,
    clearMessages,
  } = usePlaygroundState()

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config,
    parameterEnabled,
    onMessageUpdate: updateMessages,
  })

  const {
    editingMessageKey,
    handleSendMessage,
    handleRegenerateMessage,
    handleEditMessage,
    handleEditOpenChange,
    applyEdit,
    handleDeleteMessage,
  } = usePlaygroundConversation({
    messages,
    updateMessages,
    sendChat,
  })

  const handleClearMessages = () => {
    handleEditOpenChange(false)
    clearMessages()
  }

  const { isLoadingModels } = usePlaygroundOptions({
    currentGroup: config.group,
    currentModel: config.model,
    setGroups,
    setModels,
    updateConfig,
  })

  return (
    <div className='relative flex size-full min-h-0 flex-col overflow-hidden'>
      <MediaWorkspace
        config={config}
        editingMessageKey={editingMessageKey}
        groups={groups}
        isGenerating={isGenerating}
        isLoadingMessages={isLoadingMessages}
        isModelLoading={isLoadingModels}
        messages={messages}
        models={models}
        parameterEnabled={parameterEnabled}
        onClearMessages={handleClearMessages}
        onConfigChange={updateConfig}
        onDeleteMessage={handleDeleteMessage}
        onEditMessage={handleEditMessage}
        onEditOpenChange={handleEditOpenChange}
        onParameterEnabledChange={updateParameterEnabled}
        onRegenerateMessage={handleRegenerateMessage}
        onSaveEdit={applyEdit}
        onSendMessage={handleSendMessage}
        onStop={stopGeneration}
      />
    </div>
  )
}
