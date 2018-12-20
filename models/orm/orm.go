package orm

import (
	"github.com/jinzhu/gorm"
)

type (
	dbFunc func(tx *gorm.DB) error // func type which accept *gorm.DB and return error
)

func transaction(db *gorm.DB, fn dbFunc) (err error) {
	// start db transaction
	tx := db.Begin()
	defer tx.Commit()

	err = fn(tx)

	// close db transaction
	return err
}

func FindAll(db *gorm.DB, v interface{}) (err error) {
	return transaction(db, func(tx *gorm.DB) error {
		if err = tx.Find(v).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

func Create(db *gorm.DB, v interface{}) error {
	return transaction(db, func(tx *gorm.DB) (err error) {
		if !db.NewRecord(v) {
			return err
		}
		if err = tx.Create(v).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

func Save(db *gorm.DB, v interface{}) error {
	return transaction(db, func(tx *gorm.DB) (err error) {
		if db.NewRecord(v) {
			return err
		}
		if err = tx.Save(v).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

func Delete(db *gorm.DB, v interface{}) error {
	return transaction(db, func(tx *gorm.DB) (err error) {
		if err = tx.Delete(v).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

func FindOneByID(db *gorm.DB, v interface{}, id uint64) (err error) {
	return transaction(db, func(tx *gorm.DB) error {
		if err = tx.Last(v, id).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

func FindOneByQuery(db *gorm.DB, v interface{}, params map[string]interface{}) (err error) {
	return transaction(db, func(tx *gorm.DB) error {
		if err = tx.Where(params).Last(v).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

func FindByQueryMap(db *gorm.DB, v interface{}, params map[string]interface{}) (err error) {
	return transaction(db, func(tx *gorm.DB) error {
		if err = tx.Where(params).Find(v).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

func FindByQuery(db *gorm.DB, v interface{}, values ...interface{}) (err error) {
	return transaction(db, func(tx *gorm.DB) error {
		if err = tx.Where(values).Find(v).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

func FindAllOrder(db *gorm.DB, v interface{}, order string) (err error) {
	return transaction(db, func(tx *gorm.DB) error {
		if err = tx.Order(order).Find(v).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}
