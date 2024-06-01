package wrap_test

/*
func Test_Ping(t *testing.T) {
	var (
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		caddr  = netip.AddrPortFrom(test.LocIP(), 19986)
		saddr  = netip.AddrPortFrom(test.LocIP(), 8080)
		cfg    = &udp.Config{}
	)
	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum, test.PacketLoss(0.05), test.Delay(time.Millisecond*100),
	)
	eg, ctx := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		rawl, err := udp.Listen[conn.Default](test.NewMockListener(t, s), cfg)
		require.NoError(t, err)
		defer rawl.Close()
		l, err := server.WrapListener(rawl, server.Config{Logger: logger})
		require.NoError(t, err)

		s, err := fatun.NewServer[conn.Default](func(s *fatun.Server) {
			s.Logger = logger
			s.Listener = l
			s.Senders = []fatun.Sender{&MockSender{}}
		})
		require.NoError(t, err)

		err = s.Serve()
		require.NoError(t, err)

		time.Sleep(time.Hour)
		return nil
	})

	eg.Go(func() error {
		raw, err := udp.NewConn[conn.Default](c, netip.AddrPort{}, netip.AddrPort{}, cfg)
		require.NoError(t, err)
		_, err = raw.BuiltinConn(ctx)
		require.NoError(t, err)

		wc, err := client.WrapConn(raw, &client.Config{Logger: logger})
		require.NoError(t, err)

		c, err := fatun.NewClient[conn.Default](func(c *fatun.Client) {
			c.Logger = logger
			c.Conn = wc
			c.Capturer = &MockCapture{}
		})
		require.NoError(t, err)
		defer c.Close()

		c.Run()

		for {
			d, err := wc.Ping()
			require.NoError(t, err)

			fmt.Println("ping", d)

			time.Sleep(time.Second)
		}
	})

	eg.Wait()
}

type MockSender struct{}

func (m *MockSender) Recv(ip *packet.Packet) error { select {} }
func (m *MockSender) Send(ip *packet.Packet) error { return nil }
func (m *MockSender) Close() error                 { panic("") }

type MockCapture struct{}

func (m *MockCapture) Capture(ip *packet.Packet) error { select {} }
func (m *MockCapture) Inject(ip *packet.Packet) error  { return nil }
func (m *MockCapture) Close() error                    { panic("") }
*/
