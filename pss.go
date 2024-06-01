package accelerator

// type PrevSegmets [][]byte

// type ErrPrevPacketInvalid int

// func (e ErrPrevPacketInvalid) Error() string {
// 	return fmt.Sprintf("previous pakcet %d is invalid", e)
// }

// func (pss PrevSegmets) Size() int {
// 	n := 0
// 	for _, e := range pss {
// 		n += len(e)
// 	}
// 	return n
// }

// func (pss PrevSegmets) Marshal(to string) error {
// 	fh, err := os.Create(to)
// 	if err != nil {
// 		return err
// 	}
// 	defer fh.Close()

// 	var dst []byte
// 	for i, e := range pss {
// 		n := hex.EncodedLen(len(e))
// 		dst = slices.Grow(dst, n+1)
// 		dst = dst[:n+1]

// 		hex.Encode(dst, e)
// 		if i != len(pss)-1 {
// 			dst[n] = '\n'
// 			n = n + 1
// 		}
// 		if _, err = fh.Write(dst[:n]); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// func (pss *PrevSegmets) Unmarshal(from string) error {
// 	fh, err := os.Open(from)
// 	if err != nil {
// 		return errors.WithStack(err)
// 	}
// 	defer fh.Close()

// 	data, err := io.ReadAll(fh)
// 	if err != nil {
// 		return errors.WithStack(err)
// 	}

// 	ps := bytes.Split(data, []byte{'\n'})
// 	for i := range ps {
// 		n, err := hex.Decode(ps[i], ps[i])
// 		if err != nil {
// 			return errors.WithStack(err)
// 		}
// 		ps[i] = ps[i][:n]
// 	}
// 	(*pss) = ps

// 	return nil
// }

// func (pss PrevSegmets) Client(ctx context.Context, conn net.Conn) (key crypto.Key, err error) {

// 	for i := 0; i < len(pss); i++ {
// 		if i%2 == 0 {
// 			_, err := conn.Write(pss[i])
// 			if err != nil {
// 				select {
// 				case <-ctx.Done():
// 					return crypto.Key{}, errors.WithStack(ctx.Err())
// 				default:
// 					return crypto.Key{}, errors.WithStack(err)
// 				}
// 			}
// 		} else {
// 			var b = make([]byte, len(pss[i]))

// 			if _, err := io.ReadFull(conn, b); err != nil {
// 				select {
// 				case <-ctx.Done():
// 					return crypto.Key{}, errors.WithStack(ctx.Err())
// 				default:
// 					return crypto.Key{}, errors.WithStack(err)
// 				}
// 			}
// 			if !bytes.Equal(b, pss[i]) {
// 				return crypto.Key{}, ErrPrevPacketInvalid(i)
// 			}
// 		}
// 	}
// 	return crypto.Key{}, nil
// }

// func (pss PrevSegmets) Server(ctx context.Context, conn net.Conn) (key crypto.Key, err error) {
// 	for i := 0; i < len(pss); i++ {
// 		if i%2 == 0 {
// 			var b = make([]byte, len(pss[i]))

// 			if _, err := io.ReadFull(conn, b); err != nil {
// 				select {
// 				case <-ctx.Done():
// 					return crypto.Key{}, errors.WithStack(ctx.Err())
// 				default:
// 					return crypto.Key{}, errors.WithStack(err)
// 				}
// 			}
// 			if !bytes.Equal(b, pss[i]) {
// 				return crypto.Key{}, ErrPrevPacketInvalid(i)
// 			}
// 		} else {
// 			_, err := conn.Write(pss[i])
// 			if err != nil {
// 				select {
// 				case <-ctx.Done():
// 					return crypto.Key{}, errors.WithStack(ctx.Err())
// 				default:
// 					return crypto.Key{}, errors.WithStack(err)
// 				}
// 			}
// 		}
// 	}
// 	return crypto.Key{}, nil
// }
