// UniqueNumbersGenerator.go -- генерирует limits чисел (каждое принадлежит интервалу [0, limits)) с помощью
// threads горутин
package main

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

// пушит случайные числа в интервале [0, n) в переданный канал ch
// прекращает работу по сигналу отмены контекста ctx
// логирует своё завершение (для этого необходим id)
func producer(ctx context.Context, n, id int, ch chan<- int) {
	// создаём новый псевдослучайный генератор чисел
	src := rand.NewSource(time.Now().Unix())
	rnd := rand.New(src)
	for {
		select {
		// слушаем контекст
		case <-ctx.Done(): // в случае сигнала отмены
			{
				// логируем завершение работы продюсера
				fmt.Printf("[Producer #%d] exit\n", id)
				// закрываем канал
				close(ch)
				// завершаем работу
				return
			}
		default: // в противном случае пушим случайное число в канал
			ch <- rnd.Intn(n)
		}

	}
}

// слушает каналы из chans, записываем limits уникальных чисел в out
func consumer(limits int, chans []chan int, out chan int) {
	// отображение: int -> struct{}
	// позволяет использовать его как множество
	unique := make(map[int]struct{})
	for {
		// флаг, отвечающий на вопрос: работают ли producer-ы
		producersWorks := false
		// пробегаемся по каналам
		for _, ch := range chans {
			// слушаем текущий канал ch
			num, ok := <-ch
			// если канал открыт
			if ok {
				// значит producer-ы всё ещё работают
				producersWorks = true
				// ok - есть ли в unique num?
				_, ok = unique[num]
				if !ok {
					// вставка num в unique
					unique[num] = struct{}{}
					// запись num в канал
					out <- num
				}
			}
		}
		// если producer-ы завершили свою работу
		if !producersWorks {
			// логируем завершение работы consumer-а
			fmt.Println("[Consumer] exit")
			// закрываем канал
			close(out)
			// завершаем работу
			return
		}
	}
}

// экспортируемая функция для взаимодействия с модулем
// запускает threads producer-ов и одного consumer-а, пушит в out уникальные числа
func Handler(mainCtx context.Context, limits, threads int, out chan int) {
	// контекст для завершения producer-ов
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// запоминаем каналы, передаваемые producer-ам
	chans := []chan int{}
	// запуск threads producer-ов
	for i := 0; i < threads; i++ {
		// создаём новый канал для текущего producer-а
		curr := make(chan int)
		// кладём его в chans
		chans = append(chans, curr)
		// запуск producer-а, с передачей id для логирования
		go producer(ctx, limits, i, curr)
	}
	// канал для чисел от consumer-а
	cnsmrCh := make(chan int)
	// призрак, для завершения consumer
	ghost := func() {
		for {
			_, ok := <-cnsmrCh
			if !ok {
				fmt.Println("[Handler] exit")
				return
			}
		}
	}
	// запуск consumer-а
	go consumer(limits, chans, cnsmrCh)
	// счётчик отправленных чисел
	cnt := 0
	// запускаем прослушку аварийного контекста
	for {
		select {
		case <-mainCtx.Done():
			{
				// логируем сигнал отмены контекста
				fmt.Println("mainContext cancel signal")
				go ghost()
				return
			}
		default:
			{
				// если ещё не отправил limits чисел
				if cnt < limits {
					out <- <-cnsmrCh
					cnt++
				} else {
					go ghost()
					return
				}
			}
		}
	}
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			time.Sleep(time.Second)
			fmt.Println(runtime.NumGoroutine())
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	limits, threads := 500, 1
	out := make(chan int)
	go Handler(ctx, limits, threads, out)
	go func() {
		for i := 0; i < limits; i++ {
			if i > limits/2 {
				cancel()
				<-out
				return
			}
			<-out
		}
	}()
	wg.Wait()
}
