package grokvideo

var ModelList = []string{"grok-image-video", "grok-video-1.5"}

var grokVideo15SupportedSeconds = map[int]struct{}{
	4: {}, 6: {}, 8: {}, 10: {}, 12: {}, 15: {},
}

const ChannelName = "grok-video"
