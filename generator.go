// UniqueNumbersGenerator.go -- генерирует limits чисел (каждое принадлежит интервалу [0, limits)) с помощью
// threads горутин
package generator

import (
	"context"
	"fmt"
	"math/rand"
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
func consumer(cancel context.CancelFunc, limits int, chans []chan int, out chan int)  {
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
		// если записали все числа
		if len(unique) == limits {
			// логируем сигнал отмены контекста
			fmt.Println("context cancel signal")
			cancel()
			// почистим каналы producer-ов, чтобы они заметили сигнал отмены
			for _, ch := range chans {
				// слушаем текущий канал ch
				_ = <-ch
			}
			// producer-ы завершат свою работу
			producersWorks = false
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
    // WaitGroup, отвечающая за выполнение горутин
	var wg sync.WaitGroup
	// будет выполнено threads producer-ов и один consumer
    wg.Add(threads + 1)
	// запоминаем каналы, передаваемые producer-ам
    chans := []chan int{}
	// запуск threads producer-ов
    for i := 0; i < threads; i++ {
		// создаём новый канал для текущего producer-а 
        curr := make(chan int)
		// кладём его в chans
        chans = append(chans, curr)
		// запуск producer-а, с передачей id для логирования
        go func(i int) {
			// кладём в defer stack выполнение текущего producer-а
            defer wg.Done()
            producer(ctx, limits, i, curr)
        }(i)
    }
	// запуск consumer-а
	consumerWorks := true
    go func() {
        defer wg.Done()
        consumer(cancel, limits, chans, out)
		consumerWorks = false
    }()
	// запускаем прослушку аварийного контекста
	for {
		select {
			case <-mainCtx.Done(): {
				// логируем сигнал отмены контекста
				fmt.Println("mainContext cancel signal")
				cancel()
				fmt.Println("[Handler] exit")
				return
			}
			default:{
				if !consumerWorks {
					fmt.Print("[Handler] exit")
					return
				}
			}
		}
	}
	
}

