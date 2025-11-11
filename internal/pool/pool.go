package pool

import (
	"sync"
)

// Resettable — любой тип, где есть метод Reset().
// Важно: если Reset объявлен на *T, то удовлетворяет именно *T (указатель).
type Resettable interface {
	Reset()
}

// Pool — generics-обёртка над sync.Pool для типов с Reset().
type Pool[T Resettable] struct {
	p   sync.Pool
	new func() T
}

// New создаёт пул. newFn — фабрика объектов, когда пул пуст.
// Пример: New(func() *MyType { return &MyType{} })
func New[T Resettable](newFn func() T) *Pool[T] {
	pl := &Pool[T]{new: newFn}
	pl.p.New = func() any { return newFn() }
	return pl
}

// Get возвращает объект из пула (или создаёт новый через фабрику).
func (pl *Pool[T]) Get() T {
	if v := pl.p.Get(); v != nil {
		return v.(T)
	}
	// На всякий случай, если p.New не задан или вернул nil:
	if pl.new != nil {
		return pl.new()
	}
	var zero T
	return zero
}

// Put кладёт объект обратно в пул.
// Перед возвратом обязательно сбрасываем состояние.
func (pl *Pool[T]) Put(v T) {
	// Если T — указатель, допустим nil — просто игнорируем.
	if isNil(v) {
		return
	}
	v.Reset()
	pl.p.Put(v)
}

// isNil — универсальная проверка на nil для случаев, когда T — указатель/интерфейс.
// Для значимых типов (struct-значение) всегда false.
func isNil[T any](v T) bool {
	var i any = v
	return i == nil
}
