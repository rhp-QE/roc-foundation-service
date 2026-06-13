package push

import backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"

type routeSummary struct {
	successCount int32
	staleCount   int32
	noopCount    int32
	failCount    int32
	firstError   string
}

// routeSummary 汇总单用户多连接结果；成功优先，其次系统失败，再区分 stale 和 noop。
func (s *routeSummary) success() {
	s.successCount++
}

func (s *routeSummary) stale() {
	s.staleCount++
}

func (s *routeSummary) noop() {
	s.noopCount++
}

func (s *routeSummary) fail(err string) {
	s.failCount++
	if s.firstError == "" {
		s.firstError = err
	}
}

func (s routeSummary) toPushResult(userID string) *backbon.PushResult {
	if s.successCount > 0 {
		return result(userID, true, backbon.PushStatus_PUSH_STATUS_SUCCESS, s.successCount, "")
	}
	if s.failCount > 0 {
		return result(userID, false, backbon.PushStatus_PUSH_STATUS_FAILED, 0, s.firstError)
	}
	if s.staleCount > 0 {
		return result(userID, true, backbon.PushStatus_PUSH_STATUS_STALE_CLEANED, 0, "")
	}
	return result(userID, true, backbon.PushStatus_PUSH_STATUS_NOOP_OFFLINE, 0, "")
}
