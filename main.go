package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	_ "github.com/lib/pq"
)

type Request struct {
	MSISDN  string `json:"MSISDN"`
	Message string `json:"message"`
}

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("postgres", "postgres://smoqqy:tomioka@localhost:5432/zet_check_balance_db?sslmode=disable")
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	defer db.Close()

	http.HandleFunc("/", handleRequest)
	log.Fatal(http.ListenAndServe(":7000", nil))
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Метод %s не поддерживается", r.Method)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Ошибка чтения тела запроса: %v", err)
		return
	}

	var req Request
	err = json.Unmarshal(body, &req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Ошибка декодирования JSON: %v", err)
		return
	}

	msisdnA := req.MSISDN
	var msisdnB string
	index := strings.Index(req.Message, "*130*")
	if index != -1 {
		msisdnB = req.Message[index+5 : len(req.Message)-1]
		fmt.Println("Значение после *130*:", msisdnB)
	} else {
		fmt.Println("Значение не найдено.")
	}

	fmt.Printf("Полученные данные: MSISDN=%s, Message=%s\n", msisdnA, req.Message)

	/*if err := addOrUpdateSubscriberA(msisdn, msisdnB); err != nil {
		log.Printf("Ошибка добавления/обновления абонента A: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Ошибка добавления/обновления абонента A: %v", err)
		return
	}*/
	acceptSubB("992907103137", "992334440")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Значение поля MSISDN: %s", msisdnA)
}

func addOrUpdateSubscriberA(msisdn string, msisdnB string) error {
	var subscriberAId int
	row := db.QueryRow("SELECT id FROM subscriber_a WHERE number=$1", msisdn)

	if err := row.Scan(&subscriberAId); err != nil {
		if err == sql.ErrNoRows {
			if _, err := db.Exec("INSERT INTO subscriber_a (number) VALUES ($1)", msisdn); err != nil {
				return fmt.Errorf("Ошибка добавления номера в базу данных: %v", err)
			}
			log.Println("Номер абонента A успешно добавлен в базу данных")
		} else {
			return fmt.Errorf("Ошибка выполнения запроса: %v", err)
		}
	} else {
		log.Println("Номер абонента A существует", subscriberAId)
	}

	if err := addOrUpdateSubscriberB(msisdnB, subscriberAId); err != nil {
		return err
	}

	return nil
}

func addOrUpdateSubscriberB(msisdnB string, subscriberAId int) error {
	var count int
	err := db.QueryRow("SELECT count(*) FROM service WHERE subscriber_b_number=$1", msisdnB).Scan(&count)
	if err != nil {
		return fmt.Errorf("Ошибка выполнения запроса: %v", err)
	}

	if count > 0 {
		log.Println("Номер абонента B существует")
	} else {
		row := db.QueryRow("SELECT subscriber_b_number FROM service WHERE subscriber_b_number=$1", msisdnB)
		var subsNumberBFromDB string
		if err := row.Scan(&subsNumberBFromDB); err != nil {
			if err == sql.ErrNoRows {
				fmt.Println(msisdnB)
				if _, err := db.Exec("INSERT INTO service (subscriber_a_id, subscriber_b_number, is_accept) VALUES ($1, $2, $3)", subscriberAId, msisdnB, false); err != nil {
					return fmt.Errorf("Ошибка добавления номера в базу данных: %v", err)
				}
				log.Println("Номер абонента B успешно добавлен в базу данных")
			} else {
				return fmt.Errorf("Ошибка выполнения запроса: %v", err)
			}
		} else {
			log.Println("Номер абонента B существует")
		}
	}

	return nil
}

func acceptSubB(msisdnB string, msisdnA string) error {
	fmt.Println("start")
	var id int
	var is_accept bool
	var subscriber_b_number string
	var subscriber_a_number string
	fmt.Println(msisdnA, "   ", msisdnB)
	err := db.QueryRow("select s.id from service join subscriber_a sa on sa.id = s.subscriber_a_id where is_accept = false and subscriber_b_number = $1 and sa.number = $2", msisdnB, msisdnA).Scan(&id)
	fmt.Println(id, is_accept, subscriber_a_number, subscriber_b_number)
	fmt.Println("after_select")

	if err != nil {
		return fmt.Errorf("Ошибка выполнения запроса: %v", err)
	}
	fmt.Println("clear_select")
	fmt.Println(id, is_accept, subscriber_a_number, subscriber_b_number)

	return nil
}
