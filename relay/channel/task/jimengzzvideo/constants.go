package jimengzzvideo

var ModelList = []string{
	"video-ds-2.0",
	"video-ds-2.0-fast",
	"as-sd2.0-fast",
	"seedance2.5",
	"minimax-h3",
	"kling-video-v3-omni",
	"kling-video-v3",
	"kling-video-v3-turbo",
}

var ChannelName = "jimeng-zz-video"

type modelCapabilities struct {
	MinSeconds int
	MaxSeconds int
	MaxImages  int
	MaxVideos  int
	MaxAudios  int
	Resolutions map[string]struct{}
}

var defaultModelCapabilities = modelCapabilities{
	MaxImages: 4,
	MaxVideos: 3,
	MaxAudios: 1,
}

var modelCapabilitiesByName = map[string]modelCapabilities{
	"seedance2.5": {
		MinSeconds: 4,
		MaxSeconds: 30,
		MaxImages:  30,
		MaxVideos:  10,
		MaxAudios:  10,
		Resolutions: map[string]struct{}{
			"480p": {},
			"720p": {},
		},
	},
	"minimax-h3": {
		MinSeconds: 5,
		MaxSeconds: 15,
		MaxImages:  5,
		MaxVideos:  3,
		MaxAudios:  1,
		Resolutions: map[string]struct{}{
			"2k": {},
		},
	},
	"kling-video-v3-omni": {
		MinSeconds: 3,
		MaxSeconds: 15,
		Resolutions: map[string]struct{}{
			"720p": {}, "1080p": {}, "4k": {},
		},
	},
	"kling-video-v3": {
		MinSeconds: 3,
		MaxSeconds: 15,
		Resolutions: map[string]struct{}{
			"720p": {}, "1080p": {}, "4k": {},
		},
	},
	"kling-video-v3-turbo": {
		MinSeconds: 3,
		MaxSeconds: 15,
		Resolutions: map[string]struct{}{
			"720p": {}, "1080p": {},
		},
	},
}

func getModelCapabilities(modelName string) modelCapabilities {
	if capabilities, ok := modelCapabilitiesByName[modelName]; ok {
		return capabilities
	}
	return defaultModelCapabilities
}
