package service

import "math"

// paging 归一化分页参数并计算 SQL offset；offset 用 int64 计算并限制在 int32 上限内，避免溢出产生负偏移。
func paging(page, pageSize int32) (int32, int32, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (int64(page) - 1) * int64(pageSize)
	if offset > math.MaxInt32 {
		offset = math.MaxInt32
	}
	return page, pageSize, int(offset)
}
