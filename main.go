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

	msisdn := req.MSISDN
	var msisdnB string
	index := strings.Index(req.Message, "*130*")
	if index != -1 {
		msisdnB := req.Message[index+5 : len(req.Message)-1]
		fmt.Println("Значение после *130*:", msisdnB)
	} else {
		fmt.Println("Значение не найдено.")
	}

	fmt.Printf("Полученные данные: MSISDN=%s, Message=%s\n", msisdn, req.Message)

	if err := addOrUpdateSubscriberA(msisdn); err != nil {
		log.Printf("Ошибка добавления/обновления абонента A: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Ошибка добавления/обновления абонента A: %v", err)
		return
	}

	if err := addOrUpdateSubscriberB(msisdnB, subscriberAId); err != nil {
		log.Printf("Ошибка добавления/обновления абонента B: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Ошибка добавления/обновления абонента B: %v", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Значение поля MSISDN: %s", msisdn)
}

func addOrUpdateSubscriberA(msisdn string) error {
	var subsNumberA string
	var subscriberAId int
	row := db.QueryRow("SELECT id, number FROM subscriber_a WHERE number=$1", msisdn)

	if err := row.Scan(&subscriberAId, &subsNumberA); err != nil {
		if err == sql.ErrNoRows {
			if _, err := db.Exec("INSERT INTO subscriber_a (number) VALUES ($1)", msisdn); err != nil {
				return fmt.Errorf("Ошибка добавления номера в базу данных: %v", err)
			}
			log.Println("Номер абонента A успешно добавлен в базу данных")
			addOrUpdateSubscriberB(msisdnB, subscriberAId)

		} else {
			return fmt.Errorf("Ошибка выполнения запроса: %v", err)
		}
	} else {
		log.Println("Номер абонента A существует", subscriberAId)
	}

	return nil
}

func addOrUpdateSubscriberB(subsNumberB string, subscriberAId int) error {
	var count int
	err := db.QueryRow("SELECT count(*) FROM service WHERE subscriber_b_number=$1", subsNumberB).Scan(&count)
	if err != nil {
		return fmt.Errorf("Ошибка выполнения запроса: %v", err)
	}

	if count > 0 {
		log.Println("Номер абонента B существует")
	} else {
		row := db.QueryRow("SELECT subscriber_b_number FROM service WHERE subscriber_b_number=$1", subsNumberB)
		var subsNumberBFromDB string
		if err := row.Scan(&subsNumberBFromDB); err != nil {
			if err == sql.ErrNoRows {
				fmt.Println(subsNumberB)
				if _, err := db.Exec("INSERT INTO service (subscriber_b_number) VALUES ($1)", subsNumberB); err != nil {
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
