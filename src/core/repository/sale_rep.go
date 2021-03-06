/**
 * Copyright 2014 @ S1N1 Team.
 * name :
 * author : jarryliu
 * date : 2013-12-08 11:09
 * description :
 * history :
 */

package repository

import (
	"database/sql"
	"fmt"
	"github.com/atnet/gof/algorithm/iterator"
	"github.com/atnet/gof/db"
	"go2o/src/core/domain/interface/sale"
	"go2o/src/core/domain/interface/valueobject"
	saleImpl "go2o/src/core/domain/sale"
	"go2o/src/core/infrastructure/format"
	"go2o/src/core/infrastructure/log"
)

var _ sale.ISaleRep = new(saleRep)

type saleRep struct {
	db.Connector
	cache   map[int]sale.ISale
	_tagRep sale.ISaleTagRep
}

func NewSaleRep(c db.Connector, saleTagRep sale.ISaleTagRep) sale.ISaleRep {
	return (&saleRep{
		Connector: c,
		_tagRep:   saleTagRep,
	}).init()
}

func (this *saleRep) init() sale.ISaleRep {
	this.cache = make(map[int]sale.ISale)
	return this
}

func (this *saleRep) GetSale(partnerId int) sale.ISale {
	v, ok := this.cache[partnerId]
	if !ok {
		v = saleImpl.NewSale(partnerId, this, this._tagRep)
		this.cache[partnerId] = v
	}
	return v
}

func (this *saleRep) GetValueItem(partnerId, itemId int) *sale.ValueItem {
	var e *sale.ValueItem = new(sale.ValueItem)
	err := this.Connector.GetOrm().GetByQuery(e, `select * FROM gs_item
			INNER JOIN gs_category c ON c.id = gs_item.category_id WHERE gs_item.id=?
			AND c.partner_id=?`, itemId, partnerId)
	if err != nil {
		return nil
	}
	return e
}

func (this *saleRep) GetItemByIds(ids ...int) ([]*sale.ValueItem, error) {
	//todo: partnerId
	var items []*sale.ValueItem

	//todo:改成database/sql方式，不使用orm
	err := this.Connector.GetOrm().SelectByQuery(&items,
		`SELECT * FROM gs_item WHERE id IN (`+format.GetCategoryIdStr(ids)+`)`)

	return items, err
}

func (this *saleRep) SaveValueItem(v *sale.ValueItem) (int, error) {
	orm := this.Connector.GetOrm()
	if v.Id <= 0 {
		_, id, err := orm.Save(nil, v)
		return int(id), err
	} else {
		_, _, err := orm.Save(v.Id, v)
		return v.Id, err
	}
}

func (this *saleRep) GetPagedOnShelvesItem(partnerId int, catIds []int, start, end int) (total int, e []*sale.ValueItem) {
	var sql string

	var catIdStr string = format.GetCategoryIdStr(catIds)
	sql = fmt.Sprintf(`SELECT * FROM gs_item INNER JOIN gs_category ON gs_item.category_id=gs_category.id
		WHERE partner_id=%d AND gs_category.id IN (%s) AND on_shelves=1 LIMIT %d,%d`, partnerId, catIdStr, start, (end - start))

	this.Connector.ExecScalar(fmt.Sprintf(`SELECT COUNT(0) FROM gs_item INNER JOIN gs_category ON gs_item.category_id=gs_category.id
		WHERE partner_id=%d AND gs_category.id IN (%s) AND on_shelves=1`, partnerId, catIdStr), &total)

	e = []*sale.ValueItem{}
	this.Connector.GetOrm().SelectByQuery(&e, sql)

	return total, e
}

func (this *saleRep) DeleteItem(partnerId, itemId int) error {
	_, _, err := this.Connector.Exec(`
		DELETE f,f2 FROM gs_item AS f
		INNER JOIN gs_category AS c ON f.category_id=c.id
		INNER JOIN gs_itemprop as f2 ON f2.id=f.id
		WHERE f.id=? AND c.partner_id=?`, itemId, partnerId)
	return err
}

//获取食物数量
//todo: 还未使用
func (this *saleRep) FoodItemsCount(partnerId, cid int) (count int) {
	this.Connector.QueryRow(`
		SELECT COUNT(0) FROM gs_item f
	INNER JOIN gs_category c ON f.category_id = c.id
	 where c.partner_id = ?
	AND (cid == -1 OR cid = ?)
	`, func(r *sql.Row) {
		r.Scan(count)
	}, partnerId, cid)
	return count
}

func (this *saleRep) SaveCategory(v *sale.ValueCategory) (int, error) {
	orm := this.Connector.GetOrm()
	if v.Id <= 0 {
		_, _, err := orm.Save(nil, v)
		if err == nil {
			this.Connector.ExecScalar(`SELECT MAX(id) FROM gs_category`, &v.Id)
		}
		return v.Id, err
	} else {
		_, _, err := orm.Save(v.Id, v)
		return v.Id, err
	}
}

func (this *saleRep) DeleteCategory(partnerId, id int) error {
	//删除子类
	_, _, err := this.Connector.Exec("DELETE FROM gs_category WHERE partner_id=? AND parent_id=?",
		partnerId, id)

	//删除分类
	_, _, err = this.Connector.Exec("DELETE FROM gs_category WHERE partner_id=? AND id=?",
		partnerId, id)

	//清理项
	this.Connector.Exec(`DELETE FROM gs_item WHERE Cid NOT IN
		(SELECT Id FROM gs_category WHERE partner_id=?)`, partnerId)

	return err
}

func (this *saleRep) GetCategory(partnerId, id int) *sale.ValueCategory {
	var e *sale.ValueCategory = new(sale.ValueCategory)
	err := this.Connector.GetOrm().Get(id, e)
	if err != nil {
		log.PrintErr(err)
		return nil
	}

	if e.PartnerId != partnerId {
		return nil
	}

	return e
}

func (this *saleRep) GetCategories(partnerId int) []*sale.ValueCategory {
	var e []*sale.ValueCategory = []*sale.ValueCategory{}
	err := this.Connector.GetOrm().Select(&e, "partner_id=? ORDER BY id ASC", partnerId)
	if err != nil {
		log.PrintErr(err)
	}
	return e
}

// 获取与栏目相关的栏目
func (this *saleRep) GetRelationCategories(partnerId, categoryId int) []*sale.ValueCategory {
	var all []*sale.ValueCategory = this.GetCategories(partnerId)
	var newArr []*sale.ValueCategory = []*sale.ValueCategory{}
	var isMatch bool
	var pid int
	var l int = len(all)

	for i := 0; i < l; i++ {
		if !isMatch && all[i].Id == categoryId {
			isMatch = true
			pid = all[i].ParentId
			newArr = append(newArr, all[i])
			i = -1
		} else {
			if all[i].Id == pid {
				newArr = append(newArr, all[i])
				pid = all[i].ParentId
				i = -1
				if pid == 0 {
					break
				}
			}
		}
	}
	return newArr
}

// 获取子栏目
func (this *saleRep) GetChildCategories(partnerId, categoryId int) []*sale.ValueCategory {
	var all []*sale.ValueCategory = this.GetCategories(partnerId)
	var newArr []*sale.ValueCategory = []*sale.ValueCategory{}

	var cdt iterator.Condition = func(v, v1 interface{}) bool {
		return v1.(*sale.ValueCategory).ParentId == v.(*sale.ValueCategory).Id
	}
	var start iterator.WalkFunc = func(v interface{}, level int) {
		c := v.(*sale.ValueCategory)
		if c.Id != categoryId {
			newArr = append(newArr, c)
		}
	}

	var arr []interface{} = make([]interface{}, len(all))
	for i, _ := range arr {
		arr[i] = all[i]
	}

	iterator.Walk(arr, &sale.ValueCategory{Id: categoryId}, cdt, start, nil, 1)

	return newArr
}

// 获取商品
func (this *saleRep) GetValueGoods(itemId int, skuId int) *sale.ValueGoods {
	var e *sale.ValueGoods = new(sale.ValueGoods)
	if this.Connector.GetOrm().GetBy(e, "item_id=? AND sku_id=?", itemId, skuId) == nil {
		return e
	}
	return nil
}

// 获取商品
func (this *saleRep) GetValueGoodsById(goodsId int) *sale.ValueGoods {
	var e *sale.ValueGoods = new(sale.ValueGoods)
	if this.Connector.GetOrm().Get(goodsId, e) == nil {
		return e
	}
	return nil
}

// 根据SKU获取商品
func (this *saleRep) GetValueGoodsBySku(itemId, sku int) *sale.ValueGoods {
	var e *sale.ValueGoods = new(sale.ValueGoods)
	if this.Connector.GetOrm().GetBy(e, "item_id=? AND sku_id=?", itemId, sku) == nil {
		return e
	}
	return nil
}

// 根据编号获取商品
func (this *saleRep) GetGoodsByIds(ids ...int) ([]*valueobject.Goods, error) {
	var items []*valueobject.Goods
	err := this.Connector.GetOrm().SelectByQuery(&items,
		`SELECT * FROM gs_goods INNER JOIN gs_item ON gs_goods.item_id=gs_item.id
	 WHERE gs_goods.id IN (`+format.GetCategoryIdStr(ids)+`)`)

	return items, err
}

// 获取会员价
func (this *saleRep) GetGoodsLevelPrice(goodsId int) []*sale.MemberPrice {
	var items []*sale.MemberPrice
	if this.Connector.GetOrm().SelectByQuery(&items,
		`SELECT * FROM gs_member_price WHERE goods_id = ?`, goodsId) == nil {
		return items
	}
	return nil
}

// 保存会员价
func (this *saleRep) SaveGoodsLevelPrice(v *sale.MemberPrice) (id int, err error) {

	if v.Id <= 0 {
		this.Connector.ExecScalar(`SELECT MAX(id) FROM gs_member_price where goods_id=?`, &v.Id, v.GoodsId)
	}

	if v.Id > 0 {
		_, _, err = this.Connector.GetOrm().Save(v.Id, v)
		id = v.Id
	} else {
		_, _, err = this.Connector.GetOrm().Save(nil, v)
		if err == nil {
			err = this.Connector.ExecScalar(`SELECT MAX(id) FROM gs_member_price where goods_id=?`, &id, v.GoodsId)
		}
	}
	return id, err
}

// 移除会员价
func (this *saleRep) RemoveGoodsLevelPrice(id int) error {
	return this.Connector.GetOrm().DeleteByPk(sale.MemberPrice{}, id)
}

// 保存商品
func (this *saleRep) SaveValueGoods(v *sale.ValueGoods) (id int, err error) {
	if v.Id > 0 {
		_, _, err = this.Connector.GetOrm().Save(v.Id, v)
		id = v.Id
	} else {
		_, _, err = this.Connector.GetOrm().Save(nil, v)
		if err == nil {
			err = this.Connector.ExecScalar(`SELECT MAX(id) FROM gs_goods where items_id=?`, &id, v.ItemId)
		}
	}
	return id, err

}

// 获取在货架上的商品
func (this *saleRep) GetPagedOnShelvesGoods(partnerId int, catIds []int, start, end int) (total int, e []*valueobject.Goods) {
	var sql string

	var catIdStr string = format.GetCategoryIdStr(catIds)

	this.Connector.ExecScalar(fmt.Sprintf(`SELECT COUNT(0) FROM gs_goods INNER JOIN gs_item ON gs_item.id = gs_goods.item_id
		 INNER JOIN gs_category ON gs_item.category_id=gs_category.id
		 WHERE gs_category.partner_id=? AND gs_category.id IN (%s) AND gs_item.state=1
		 AND gs_item.on_shelves=1`, catIdStr), &total, partnerId)

	e = []*valueobject.Goods{}
	if total > 0 {
		sql = fmt.Sprintf(`SELECT * FROM gs_goods INNER JOIN gs_item ON gs_item.id = gs_goods.item_id
		 INNER JOIN gs_category ON gs_item.category_id=gs_category.id
		 WHERE gs_category.partner_id=? AND gs_category.id IN (%s) AND gs_item.state=1
		 AND gs_item.on_shelves=1 LIMIT %d,%d`, catIdStr, start, (end - start))

		this.Connector.GetOrm().SelectByQuery(&e, sql, partnerId)
	}

	return total, e
}

// 保存快照
func (this *saleRep) SaveSnapshot(v *sale.GoodsSnapshot) (int, error) {
	var id int
	_, _, err := this.Connector.GetOrm().Save(nil, v)
	if err == nil {
		err = this.Connector.ExecScalar(`SELECT MAX(id) FROM gs_snapshot where goods_id=?`, &id, v.GoodsId)
	}

	return id, err
}

// 获取最新的商品快照
func (this *saleRep) GetLatestGoodsSnapshot(goodsId int) *sale.GoodsSnapshot {
	var e *sale.GoodsSnapshot = new(sale.GoodsSnapshot)
	if this.Connector.GetOrm().GetBy(e, "goods_id=? ORDER BY id DESC", goodsId) == nil {
		return e
	}
	return nil
}

// 获取指定的商品快照
func (this *saleRep) GetGoodsSnapshot(id int) *sale.GoodsSnapshot {
	var e *sale.GoodsSnapshot = new(sale.GoodsSnapshot)
	err := this.Connector.GetOrm().Get(id, e)
	if err != nil {
		log.PrintErr(err)
		e = nil
	}
	return e
}

// 根据Key获取商品快照
func (this *saleRep) GetGoodsSnapshotByKey(key string) *sale.GoodsSnapshot {
	var e *sale.GoodsSnapshot = new(sale.GoodsSnapshot)
	err := this.Connector.GetOrm().GetBy(e, "key=?", key)
	if err != nil {
		log.PrintErr(err)
		e = nil
	}
	return e
}
