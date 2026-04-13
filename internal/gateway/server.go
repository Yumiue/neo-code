package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"neo-code/internal/gateway/transport"
)

const (
	// MaxFrameSize 定义单条 JSON 帧允许的最大字节数，避免异常输入导致内存放大。
	MaxFrameSize int64 = 1 << 20 // 1 MiB
)

var (
	errFrameTooLarge = errors.New("frame exceeds max size")
	errFrameEmpty    = errors.New("empty frame")
)

// ServerOptions 描述网关服务启动所需的可选配置。
type ServerOptions struct {
	ListenAddress string
	Logger        *log.Logger
	listenFn      func(address string) (net.Listener, error)
}

// Server 提供基于本地 IPC 的网关服务骨架实现。
type Server struct {
	listenAddress string
	logger        *log.Logger
	listenFn      func(address string) (net.Listener, error)

	mu       sync.Mutex
	listener net.Listener
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
}

// NewServer 创建网关服务实例，并解析默认监听地址。
func NewServer(options ServerOptions) (*Server, error) {
	listenAddress := strings.TrimSpace(options.ListenAddress)
	if listenAddress == "" {
		resolved, err := transport.DefaultListenAddress()
		if err != nil {
			return nil, err
		}
		listenAddress = resolved
	}

	logger := options.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "gateway: ", log.LstdFlags)
	}

	listenFn := options.listenFn
	if listenFn == nil {
		listenFn = transport.Listen
	}

	return &Server{
		listenAddress: listenAddress,
		logger:        logger,
		listenFn:      listenFn,
		conns:         make(map[net.Conn]struct{}),
	}, nil
}

// ListenAddress 返回当前服务绑定的监听地址。
func (s *Server) ListenAddress() string {
	return s.listenAddress
}

// Serve 启动 IPC 监听并处理客户端请求。
func (s *Server) Serve(ctx context.Context, runtimePort RuntimePort) error {
	listener, err := s.listenFn(s.listenAddress)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if s.listener != nil {
		s.mu.Unlock()
		_ = listener.Close()
		return fmt.Errorf("gateway: server is already serving")
	}
	s.listener = listener
	s.mu.Unlock()

	s.logger.Printf("listening on %s", s.listenAddress)

	go func() {
		<-ctx.Done()
		_ = s.Close(context.Background())
	}()

	for {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			if errors.Is(acceptErr, net.ErrClosed) || ctx.Err() != nil || s.isClosed() {
				return nil
			}
			return fmt.Errorf("gateway: accept connection: %w", acceptErr)
		}

		if !s.registerConnection(conn) {
			_ = conn.Close()
			continue
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.untrackConnection(conn)
			s.handleConnection(ctx, conn, runtimePort)
		}()
	}
}

// Close 关闭监听器并等待所有连接处理协程退出。
func (s *Server) Close(ctx context.Context) error {
	s.mu.Lock()
	listener := s.listener
	s.listener = nil
	s.mu.Unlock()

	var closeErr error
	if listener != nil {
		closeErr = listener.Close()
	}

	for conn := range s.snapshotConnections() {
		closeErr = errors.Join(closeErr, conn.Close())
	}

	waitDone := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-ctx.Done():
		closeErr = errors.Join(closeErr, ctx.Err())
	case <-waitDone:
	}

	return closeErr
}

// isClosed 判断监听器是否已经关闭。
func (s *Server) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listener == nil
}

// snapshotConnections 返回当前连接集合的拷贝，用于关闭流程安全遍历。
func (s *Server) snapshotConnections() map[net.Conn]struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	copied := make(map[net.Conn]struct{}, len(s.conns))
	for conn := range s.conns {
		copied[conn] = struct{}{}
	}
	return copied
}

// registerConnection 在服务可用时登记连接，若网关已关闭则拒绝登记。
func (s *Server) registerConnection(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return false
	}
	s.conns[conn] = struct{}{}
	return true
}

// untrackConnection 移除已结束连接，避免连接集合持续增长。
func (s *Server) untrackConnection(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
}

// handleConnection 在单连接上循环处理消息帧并返回响应帧。
func (s *Server) handleConnection(ctx context.Context, conn net.Conn, runtimePort RuntimePort) {
	defer func() {
		_ = conn.Close()
	}()

	reader := bufio.NewReader(conn)
	encoder := json.NewEncoder(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		frame, err := decodeFrame(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			if errors.Is(err, errFrameEmpty) {
				continue
			}
			if errors.Is(err, errFrameTooLarge) {
				s.logger.Printf("decode frame failed: %v", err)
				_ = encoder.Encode(errorFrame(MessageFrame{}, NewFrameError(
					ErrorCodeInvalidFrame,
					fmt.Sprintf("frame exceeds max size %d bytes", MaxFrameSize),
				)))
				return
			}

			s.logger.Printf("decode frame failed: %v", err)
			_ = encoder.Encode(errorFrame(MessageFrame{}, NewFrameError(ErrorCodeInvalidFrame, "invalid json frame")))
			return
		}

		response := s.dispatchFrame(ctx, frame, runtimePort)
		if err := encoder.Encode(response); err != nil {
			s.logger.Printf("write frame failed: %v", err)
			return
		}
	}
}

// decodeFrame 从连接读取一条 JSON 帧并执行长度与格式校验。
func decodeFrame(reader *bufio.Reader) (MessageFrame, error) {
	payload, err := readFramePayload(reader, MaxFrameSize)
	if err != nil {
		return MessageFrame{}, err
	}

	limitedReader := &io.LimitedReader{R: bytes.NewReader(payload), N: MaxFrameSize}
	decoder := json.NewDecoder(limitedReader)

	var frame MessageFrame
	if err := decoder.Decode(&frame); err != nil {
		return MessageFrame{}, err
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return MessageFrame{}, fmt.Errorf("frame contains trailing json values")
	}

	return frame, nil
}

// readFramePayload 按换行边界读取单条帧，并限制单帧最大字节数。
func readFramePayload(reader *bufio.Reader, maxSize int64) ([]byte, error) {
	var payload []byte

	for {
		chunk, err := reader.ReadSlice('\n')
		if int64(len(payload)+len(chunk)) > maxSize {
			return nil, errFrameTooLarge
		}
		payload = append(payload, chunk...)

		if err == nil {
			break
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(payload) == 0 {
				return nil, io.EOF
			}
			break
		}
		return nil, err
	}

	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil, errFrameEmpty
	}
	return payload, nil
}

// dispatchFrame 根据请求动作生成响应帧。
func (s *Server) dispatchFrame(_ context.Context, frame MessageFrame, runtimePort RuntimePort) MessageFrame {
	_ = runtimePort

	if validationErr := ValidateFrame(frame); validationErr != nil {
		return errorFrame(frame, validationErr)
	}

	if frame.Type != FrameTypeRequest {
		return errorFrame(frame, NewFrameError(ErrorCodeInvalidFrame, "only request frames are supported"))
	}

	switch frame.Action {
	case FrameActionPing:
		return MessageFrame{
			Type:      FrameTypeAck,
			Action:    FrameActionPing,
			RequestID: frame.RequestID,
			Payload: map[string]string{
				"message": "pong",
			},
		}
	default:
		return errorFrame(frame, NewFrameError(ErrorCodeUnsupportedAction, "action is not implemented in gateway step 1"))
	}
}

// errorFrame 构建统一错误响应帧。
func errorFrame(frame MessageFrame, frameErr *FrameError) MessageFrame {
	return MessageFrame{
		Type:      FrameTypeError,
		Action:    frame.Action,
		RequestID: frame.RequestID,
		Error:     frameErr,
	}
}

var _ Gateway = (*Server)(nil)
