package session

import (
	"fmt"
)

type SPICESession struct {
	baseSession
	proxy     *SPICEProxy
	proxyAddr string
}

func NewSPICESession(id string) *SPICESession {
	return &SPICESession{
		baseSession: baseSession{
			id:          id,
			sessionType: "spice",
			status:      StatusDisconnected,
		},
	}
}

func (s *SPICESession) Connect(config ConnectionConfig) error {
	s.setStatus(StatusConnecting)

	target := fmt.Sprintf("%s:%d", config.Host, config.Port)
	if config.Port <= 0 {
		target = fmt.Sprintf("%s:5900", config.Host)
	} else if config.Port < 100 {
		// libvirt display port format: :1 -> 5901, :23 -> 5923
		target = fmt.Sprintf("%s:%d", config.Host, config.Port+5900)
	}

	s.title = fmt.Sprintf("%s (SPICE)", config.Host)

	proxy := NewSPICEProxy(target)
	addr, err := proxy.Start()
	if err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("spice proxy start: %w", err)
	}

	s.proxy = proxy
	s.proxyAddr = addr

	// Set connected immediately so frontend gets proxyAddr.
	// The actual SPICE handshake happens between spice-html5 and the SPICE server
	// through the proxy; we don't wait for it here.
	s.setStatus(StatusConnected)

	return nil
}

func (s *SPICESession) Disconnect() error {
	if s.proxy != nil {
		s.proxy.Stop()
		s.proxy = nil
	}
	s.setStatus(StatusDisconnected)
	return nil
}

func (s *SPICESession) IsConnected() bool {
	return s.Status() == StatusConnected
}

func (s *SPICESession) Resize(cols, rows int) error {
	// SPICE desktop size is managed by spice-html5/spice agent negotiation.
	return nil
}

func (s *SPICESession) Write(data []byte) error {
	// SPICE data flows through WebSocket, not this method.
	return nil
}

func (s *SPICESession) ProxyAddr() string {
	return s.proxyAddr
}
