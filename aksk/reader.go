package aksk

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// VerifyReader 流式读取并验签，不保存/还原/关闭body；调用方可用TeeReader暂存，
// 验签成功前不得执行写业务。使用MaxBodyBytes（0=8MiB，负值=不限），最多探读
// 上限外1字节；失败复用ErrBodyTooLarge/ErrBodyRead及既有鉴权哨兵。
// 先拒绝无效头/密钥/过期时间，读完后再次验证时间窗。协议仍无nonce、query不入签。
// 在每次读取前后检查请求ctx；调用方须保证底层阻塞Reader能被取消/关闭打断。
func (v *Verifier) VerifyReader(r *http.Request, body io.Reader) error {
	if _, err := v.verificationHeader(r); err != nil {
		return err
	}
	if body == nil {
		body = strings.NewReader("")
	}
	source := requestContextReader{request: r, body: body}
	hash := sha256.New()
	var input io.Reader = source
	limit := v.maxBody()
	var bounded *io.LimitedReader
	if limit > 0 {
		bounded = &io.LimitedReader{R: source, N: limit}
		input = bounded
	}
	if _, err := io.CopyBuffer(hash, input, make([]byte, 32<<10)); err != nil {
		return readerError(err)
	}
	if bounded != nil && bounded.N == 0 {
		var extra [1]byte
		n, err := io.ReadFull(source, extra[:])
		if n > 0 {
			return fmt.Errorf("%w: %d bytes", ErrBodyTooLarge, limit)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return readerError(err)
		}
	}
	if err := r.Context().Err(); err != nil {
		return readerError(err)
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return v.verifyDigest(r, digest)
}

type requestContextReader struct {
	request *http.Request
	body    io.Reader
}

func (r requestContextReader) Read(p []byte) (int, error) {
	if err := r.request.Context().Err(); err != nil {
		return 0, err
	}
	n, err := r.body.Read(p)
	if err == nil {
		err = r.request.Context().Err()
	}
	return n, err
}

func readerError(err error) error {
	var max *http.MaxBytesError
	if errors.As(err, &max) {
		return fmt.Errorf("%w: %d bytes", ErrBodyTooLarge, max.Limit)
	}
	return fmt.Errorf("%w: %w", ErrBodyRead, err)
}
