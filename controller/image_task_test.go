package controller

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAsyncImageReferencesEnforcesCanonicalContract(t *testing.T) {
	request := dto.ImageRequest{
		Images: []byte(`[{"image_url":" https://example.com/one.png "},{"image_url":"data:image/png;base64,iVBORw0KGgo="}]`),
	}

	err := normalizeAsyncImageReferences(&request)

	require.NoError(t, err)
	require.JSONEq(t, `[{"image_url":"https://example.com/one.png"},{"image_url":"data:image/png;base64,iVBORw0KGgo="}]`, string(request.Images))

	tests := []struct {
		name    string
		request dto.ImageRequest
		wantErr string
	}{
		{
			name:    "legacy image field",
			request: dto.ImageRequest{Image: []byte(`"https://example.com/input.png"`)},
			wantErr: "image is not supported for asynchronous JSON tasks; use images[].image_url",
		},
		{
			name:    "string array",
			request: dto.ImageRequest{Images: []byte(`["https://example.com/input.png"]`)},
			wantErr: "images must be an array of objects containing image_url",
		},
		{
			name: "image URL alias",
			request: dto.ImageRequest{Extra: map[string]json.RawMessage{
				"image_urls": json.RawMessage(`["https://example.com/input.png"]`),
			}},
			wantErr: "image_urls is not supported for asynchronous JSON tasks; use images[].image_url",
		},
		{
			name:    "unknown object field",
			request: dto.ImageRequest{Images: []byte(`[{"image_url":"https://example.com/input.png","role":"subject"}]`)},
			wantErr: "images[0] must contain only image_url",
		},
		{
			name:    "insecure URL",
			request: dto.ImageRequest{Images: []byte(`[{"image_url":"http://example.com/input.png"}]`)},
			wantErr: "images[0].image_url is invalid: value must be a public HTTPS URL or image data URL",
		},
		{
			name:    "data URL media type mismatch",
			request: dto.ImageRequest{Images: []byte(`[{"image_url":"data:image/jpeg;base64,iVBORw0KGgo="}]`)},
			wantErr: "images[0].image_url is invalid: data URL content does not match its image media type",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := normalizeAsyncImageReferences(&test.request)
			require.EqualError(t, err, test.wantErr)
		})
	}

	references := make([]imageTaskInputReference, maxAsyncImageReferences+1)
	for index := range references {
		references[index].ImageURL = "https://example.com/input.png"
	}
	images, err := common.Marshal(references)
	require.NoError(t, err)
	overLimit := dto.ImageRequest{Images: images}
	require.EqualError(t, normalizeAsyncImageReferences(&overLimit), "images supports at most 10 reference images")
}

func TestImageTaskResponseMapsTerminalResult(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_image",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		SubmitTime: 10,
		StartTime:  20,
		FinishTime: 30,
	}
	task.SetData(imageTaskResult{Created: 25, Data: []dto.ImageData{{Url: "https://file.example/result.png"}}})

	response := imageTaskResponse(task)
	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, int64(25), response["created"])
	data, ok := response["data"].([]dto.ImageData)
	require.True(t, ok)
	require.Len(t, data, 1)
	assert.Equal(t, "https://file.example/result.png", data[0].Url)
}

func TestBuildAsyncImageGenerationRequest(t *testing.T) {
	payload := imageTaskPayload{Request: dto.ImageRequest{Model: "gpt-image-2", Prompt: "draw"}}
	body, contentType, path, err := buildAsyncImageRequest(context.Background(), constant.TaskActionImageGeneration, payload)
	require.NoError(t, err)
	assert.Equal(t, "application/json", contentType)
	assert.Equal(t, "/v1/images/generations", path)
	encoded, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"model":"gpt-image-2"`)
}

func TestBuildAsyncImageEditWithJSONReferences(t *testing.T) {
	payload := imageTaskPayload{Request: dto.ImageRequest{
		Model:  "gemini-3.1-flash-image",
		Prompt: "edit",
		Images: []byte(`[{"image_url":"https://file.example/reference.png"}]`),
	}}
	body, contentType, path, err := buildAsyncImageRequest(context.Background(), constant.TaskActionImageEdit, payload)
	require.NoError(t, err)
	assert.Equal(t, "application/json", contentType)
	assert.Equal(t, "/v1/images/edits", path)
	encoded, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"gemini-3.1-flash-image","prompt":"edit","images":[{"image_url":"https://file.example/reference.png"}]}`, string(encoded))
}

func TestExecuteImageTaskRestoresOwnerAndSettlesExistingReservation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		tokenID        int
		returnedImages int
		billing        string
	}{
		{"playground", 0, 2, "fixed"},
		{"playground partial result", 0, 1, "fixed"},
		{"API token", 11, 2, "fixed"},
		{"API token partial result", 11, 1, "fixed"},
		{"expression image count", 0, 2, "expression"},
		{"expression partial result", 11, 1, "expression"},
		{"submitted expression task freezes pricing", 0, 1, "submitted expression"},
		{"legacy token ratios", 11, 2, "tokens"},
		{"image upstream failure refunds without negative usage", 11, 0, "failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, dialect := openTaskDialectDatabase(t, &model.User{}, &model.Channel{}, &model.Task{}, &model.Token{}, &model.Log{})
			oldDB, oldLogDB := model.DB, model.LOG_DB
			oldMain, oldLog := common.MainDatabaseType(), common.LogDatabaseType()
			oldRedis, oldMemory, oldBatch, oldConsume, oldExport := common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.DataExportEnabled
			model.DB, model.LOG_DB = db, db
			common.SetDatabaseTypes(dialect, dialect)
			common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.DataExportEnabled = false, false, false, true, false
			t.Cleanup(func() {
				model.DB, model.LOG_DB = oldDB, oldLogDB
				common.SetDatabaseTypes(oldMain, oldLog)
				common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.DataExportEnabled = oldRedis, oldMemory, oldBatch, oldConsume, oldExport
			})
			for key, value := range map[string]string{
				"R2_ACCOUNT_ID": "test-account", "R2_ACCESS_KEY_ID": "test-access", "R2_SECRET_ACCESS_KEY": "test-secret",
				"R2_BUCKET": "test-images", "R2_PUBLIC_BASE_URL": "https://images.example.com",
			} {
				t.Setenv(key, value)
			}
			const initialQuota = 100000
			const reservedQuota = 2000
			require.NoError(t, db.Create(&model.User{Id: 7, Username: "image-owner", Setting: `{"billing_preference":"wallet_only"}`, Group: "default", Status: common.UserStatusEnabled, Quota: initialQuota - reservedQuota}).Error)
			if tc.tokenID > 0 {
				require.NoError(t, db.Create(&model.Token{Id: tc.tokenID, UserId: 7, Key: "image-token", Name: "image-token", RemainQuota: initialQuota - reservedQuota, Status: common.TokenStatusEnabled}).Error)
			}
			var receivedRequest dto.ImageRequest
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := common.DecodeJson(r.Body, &receivedRequest); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if tc.billing == "failure" {
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte(`{"error":{"message":"provider failed at https://private-provider.example/generate?api_key=private-key","type":"upstream_error"}}`))
					return
				}
				data := make([]dto.ImageData, tc.returnedImages)
				for i := range data {
					data[i].Url = "https://images.example.com/result.png"
				}
				body, err := common.Marshal(map[string]any{"data": data, "created": 123, "usage": map[string]any{"input_tokens": 100, "output_tokens": 200, "total_tokens": 300}})
				if err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_, _ = w.Write(body)
			}))
			defer upstream.Close()
			require.NoError(t, db.Create(&model.Channel{Id: 1, Name: "image-channel", Type: constant.ChannelTypeOpenAI, Key: "provider-key", BaseURL: &upstream.URL, Status: common.ChannelStatusEnabled}).Error)
			task := &model.Task{TaskID: "task_image_worker", UserId: 7, ChannelId: 1, Group: "default", Platform: constant.TaskPlatformImage, Action: constant.TaskActionImageGeneration, Status: model.TaskStatusInProgress, Quota: reservedQuota,
				PrivateData: model.TaskPrivateData{TokenId: tc.tokenID, BillingSource: service.BillingSourceWallet, BillingContext: &model.TaskBillingContext{ModelPrice: 0.002, GroupRatio: 1, UsePrice: true, OriginModelName: "gpt-image-2", OtherRatios: map[string]float64{"n": 2}}},
			}
			if strings.HasSuffix(tc.billing, "expression") {
				expression := `(param("size") == "1024x1024" && header("x-render-tier") == "high" ? tier("high", fixed(0.002)) : tier("other", fixed(0.009))) * image_count`
				task.PrivateData.BillingContext.UsePrice = false
				task.PrivateData.BillingContext.TieredSnapshot = &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ExprString: expression, ExprHash: billingexpr.ExprHashString(expression), GroupRatio: 1, QuotaPerUnit: common.QuotaPerUnit, EstimatedImageCount: common.GetPointer(2), EstimatedQuotaAfterGroup: reservedQuota}
				task.PrivateData.BillingContext.BillingRequestInput = &billingexpr.RequestInput{Body: []byte(`{"size":"1024x1024","n":2}`), Headers: map[string]string{"x-render-tier": "high"}, ImageCount: common.GetPointer(2)}
				_, _, exprErr := billingexpr.RunExprWithRequest(expression, billingexpr.TokenParams{}, *task.PrivateData.BillingContext.BillingRequestInput)
				require.NoError(t, exprErr)
			} else if tc.billing == "tokens" {
				task.PrivateData.BillingContext.UsePrice = false
				task.PrivateData.BillingContext.OtherRatios = nil
				task.PrivateData.BillingContext.ImagePriceData = &types.PriceData{ModelRatio: 2, CompletionRatio: 2, CacheRatio: 1, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}}
			}
			task.SetData(imageTaskPayload{Request: dto.ImageRequest{Model: "gpt-image-2", Prompt: "draw a cat", N: common.GetPointer(uint(2))}})
			if tc.billing == "submitted expression" {
				saved := map[string]string{}
				require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error { saved[key] = value; return nil }))
				t.Cleanup(func() { require.NoError(t, config.GlobalConfig.LoadFromDB(saved)) })
				expressionJSON, err := common.Marshal(map[string]string{"gpt-image-2": task.PrivateData.BillingContext.TieredSnapshot.ExprString})
				require.NoError(t, err)
				require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"billing_setting.billing_mode": `{"gpt-image-2":"tiered_expr"}`, "billing_setting.billing_expr": string(expressionJSON), "group_ratio_setting.group_ratio": `{"default":1}`}))
				require.NoError(t, db.Model(&model.User{}).Where("id = ?", 7).Update("quota", initialQuota).Error)
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/pg/images/tasks", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat","n":2,"size":"1024x1024"}`))
				c.Request.Header.Set("Content-Type", "application/json")
				c.Request.Header.Set("Authorization", "Bearer credential-not-to-persist")
				c.Request.Header.Set("X-Render-Tier", "high")
				common.SetContextKey(c, constant.ContextKeyUserId, 7)
				user, err := model.GetUserCache(7)
				require.NoError(t, err)
				user.WriteContext(c)
				require.NoError(t, middleware.SetupContextForToken(c, &model.Token{UserId: 7, Group: "default"}))
				var channel model.Channel
				require.NoError(t, db.First(&channel, 1).Error)
				require.Nil(t, middleware.SetupContextForSelectedChannel(c, &channel, "gpt-image-2"))
				CreateImageTask(c)
				require.Equal(t, http.StatusAccepted, recorder.Code, recorder.Body.String())
				task = &model.Task{}
				require.NoError(t, db.Last(task).Error)
				require.NotNil(t, task.PrivateData.BillingContext.TieredSnapshot)
				require.NotNil(t, task.PrivateData.BillingContext.ImagePriceData)
				require.NotNil(t, task.PrivateData.BillingContext.BillingRequestInput)
				assert.NotContains(t, task.PrivateData.BillingContext.BillingRequestInput.Headers, "Authorization")
				assert.NotContains(t, string(task.Data), "credential-not-to-persist")
				assert.Equal(t, reservedQuota, task.Quota)
				// A later administrator edit must not change an accepted task's contract.
				require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"billing_setting.billing_expr": `{"gpt-image-2":"tier(\"new\", fixed(1)) * image_count"}`}))
			} else {
				require.NoError(t, db.Create(task).Error)
			}
			require.NoError(t, db.First(task, task.ID).Error)
			service.InitHttpClient()

			runErr := executeImageTask(context.Background(), task)
			if tc.billing == "failure" {
				require.Error(t, runErr)
				finishImageTaskFailure(context.Background(), task, runErr)
				finishImageTaskFailure(context.Background(), task, runErr)
				var owner model.User
				require.NoError(t, db.First(&owner, 7).Error)
				assert.Equal(t, initialQuota, owner.Quota)
				assert.Zero(t, owner.UsedQuota, "image reservations do not accumulate usage until completion")
				var channel model.Channel
				require.NoError(t, db.First(&channel, 1).Error)
				assert.Zero(t, channel.UsedQuota)
				var token model.Token
				require.NoError(t, db.First(&token, tc.tokenID).Error)
				assert.Equal(t, initialQuota, token.RemainQuota)
				assert.Zero(t, task.Quota)
				assert.NotContains(t, task.FailReason, "private-provider.example")
				assert.NotContains(t, task.FailReason, "private-key")
				assert.Contains(t, task.PrivateData.UpstreamFailReason, "private-provider.example")
				var refunds int64
				require.NoError(t, db.Model(&model.Log{}).Where("type = ?", model.LogTypeRefund).Count(&refunds).Error)
				assert.Equal(t, int64(1), refunds)
				return
			}
			require.NoError(t, runErr)

			assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), task.Status)
			assert.Equal(t, "gpt-image-2", receivedRequest.Model)
			require.NotNil(t, receivedRequest.N)
			assert.Equal(t, uint(2), *receivedRequest.N)
			var owner model.User
			require.NoError(t, db.First(&owner, 7).Error)
			actualQuota := 1000 * tc.returnedImages
			if tc.billing == "tokens" {
				actualQuota = 1000
			}
			assert.Equal(t, initialQuota-actualQuota, owner.Quota, "the worker must settle the original reservation without another pre-consume")
			assert.Equal(t, actualQuota, owner.UsedQuota)
			assert.Equal(t, actualQuota, task.Quota)
			if tc.tokenID > 0 {
				var token model.Token
				require.NoError(t, db.First(&token, tc.tokenID).Error)
				assert.Equal(t, initialQuota-actualQuota, token.RemainQuota)
			}
			var logs []model.Log
			require.NoError(t, db.Where("type = ?", model.LogTypeConsume).Find(&logs).Error)
			require.Len(t, logs, 1)
			assert.Equal(t, 7, logs[0].UserId)
			assert.Equal(t, tc.tokenID, logs[0].TokenId)
			assert.Equal(t, actualQuota, logs[0].Quota)
		})
	}
}
