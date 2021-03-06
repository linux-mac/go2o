/**
 * Copyright 2014 @ S1N1 Team.
 * name :
 * author : jarryliu
 * date : 2013-12-12 16:53
 * description :
 * history :
 */

package promotion

import (
	"go2o/src/core/infrastructure/domain"
)

var (
	ErrCanNotApplied *domain.DomainError = domain.NewDomainError(
		"name_exists", "无法应用此优惠")

	ErrExistsSamePromotionFlag *domain.DomainError = domain.NewDomainError(
		"exists_same_promotion_flag", "已存在相同的促销")
)
