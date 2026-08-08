package common

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// R2Store is enabled only when all required R2 settings are present.
type R2Store struct {
	accessKey string
	secretKey string
	endpoint  *url.URL
	bucket    string
	baseURL   string
}

func NewR2StoreFromEnv() (*R2Store, error) {
	accountID := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID"))
	accessKey := strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID"))
	secretKey := strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY"))
	bucket := strings.TrimSpace(os.Getenv("R2_BUCKET"))
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("R2_PUBLIC_BASE_URL")), "/")
	if accountID == "" || accessKey == "" || secretKey == "" || bucket == "" || baseURL == "" {
		if accountID != "" || accessKey != "" || secretKey != "" || bucket != "" || baseURL != "" {
			return nil, fmt.Errorf("R2 configuration is incomplete")
		}
		return nil, nil
	}
	endpoint, err := url.Parse(GetEnvOrDefaultString("R2_ENDPOINT", "https://"+accountID+".r2.cloudflarestorage.com"))
	if err != nil {
		return nil, fmt.Errorf("parse R2 endpoint: %w", err)
	}
	if endpoint.Scheme != "https" || endpoint.Host == "" {
		return nil, fmt.Errorf("R2 endpoint must be an HTTPS URL")
	}
	publicURL, err := url.Parse(baseURL)
	if err != nil || publicURL.Scheme != "https" || publicURL.Host == "" {
		return nil, fmt.Errorf("R2 public base URL must be an HTTPS URL")
	}
	return &R2Store{accessKey: accessKey, secretKey: secretKey, endpoint: endpoint, bucket: bucket, baseURL: baseURL}, nil
}

func (r *R2Store) Put(ctx context.Context, key, contentType string, src io.ReadSeeker, size int64) error {
	payloadHash, err := hashReader(src)
	if err != nil {
		return fmt.Errorf("hash R2 object: %w", err)
	}
	if _, err = src.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind R2 object: %w", err)
	}

	requestURL := *r.endpoint
	requestURL.Path = path.Join(requestURL.Path, r.bucket, key)
	requestURL.RawQuery = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, requestURL.String(), src)
	if err != nil {
		return fmt.Errorf("create R2 upload request: %w", err)
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Cache-Control", "public, max-age=3600")
	if err := r.sign(req, payloadHash, time.Now()); err != nil {
		return err
	}

	response, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return fmt.Errorf("upload R2 object: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("upload R2 object: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (r *R2Store) URL(key string) string { return r.baseURL + "/" + strings.TrimLeft(key, "/") }

func R2DatePrefix(now time.Time) string { return "temporary/" + now.Format("2006/01/02") + "/" }

func hashReader(src io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, src); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (r *R2Store) sign(req *http.Request, payloadHash string, now time.Time) error {
	now = now.UTC()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	canonicalHeaders := "host:" + req.URL.Host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := req.Method + "\n" + req.URL.EscapedPath() + "\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHash
	scope := date + "/auto/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex(canonicalRequest)
	signingKey := hmacSHA256([]byte("AWS4"+r.secretKey), date)
	signingKey = hmacSHA256(signingKey, "auto")
	signingKey = hmacSHA256(signingKey, "s3")
	signingKey = hmacSHA256(signingKey, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+r.accessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
	return nil
}

func sha256Hex(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key []byte, value string) []byte {
	hash := hmac.New(sha256.New, key)
	_, _ = hash.Write([]byte(value))
	return hash.Sum(nil)
}
