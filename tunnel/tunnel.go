package tunnel

import (
	"context"
	"io"
	"nat/stderr"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

func Copy(ctx context.Context, dst, src io.ReadWriteCloser, log *logrus.Entry) error {
	defer dst.Close()
	defer src.Close()

	var errCh = make(chan error, 1)
	defer close(errCh)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		n, err := io.Copy(dst, src)
		if os.IsTimeout(err) {
			if err.Error() == NoRecentNetworkActivity {
				errCh <- stderr.New("closed", NoRecentNetworkActivity)
				return
			}
		}
		if err != nil {
			log.Errorf("copy error:%v", err)
		} else {
			log.Debugf("end copy %d", n)
		}
	}()
	go func() {
		defer wg.Done()
		n, err := io.Copy(src, dst)
		if os.IsTimeout(err) {
			if err.Error() == NoRecentNetworkActivity {
				errCh <- stderr.New("closed", NoRecentNetworkActivity)
				return
			}
		}
		if err != nil {
			log.Errorf("copy error:%v", err)
		} else {
			log.Debugf("end copy %d", n)
		}

	}()

	go func() {
		wg.Wait()
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
