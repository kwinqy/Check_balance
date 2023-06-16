package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
)

type Envelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    Body     `xml:"Body"`
}

type Body struct {
	XMLName                xml.Name               `    xml:"Body"`
	GetClntBalanceResponse GetClntBalanceResponse `xml:"getClntBalanceResponse"`
}

type GetClntBalanceResponse struct {
	XMLName xml.Name `xml:"getClntBalanceResponse"`
	Return  Return   `xml:"return"`
}

type Return struct {
	XMLName     xml.Name    `xml:"return"`
	ResultCode  string      `xml:"resultCode"`
	ClntBalance ClntBalance `xml:"clntBalance"`
}

type ClntBalance struct {
	XMLName       xml.Name      `xml:"clntBalance"`
	ClientBalance ClientBalance `xml:"clientBalance"`
	BalanceDict   BalanceDict   `xml:"balanceDict"`
}

type ClientBalance struct {
	XMLName      xml.Name `xml:"clientBalance"`
	ClntId       string   `xml:"clntId"`
	ClntBalId    string   `xml:"clntBalId"`
	BalanceId    string   `xml:"balanceId"`
	Priority     string   `xml:"priority"`
	BalanceSum   string   `xml:"balanceSum"`
	StartDate    string   `xml:"startDate"`
	BalanceLevel string   `xml:"balanceLevel"`
	FreezeDay    string   `xml:"freezeDay"`
	IncDate      string   `xml:"incDate"`
	DecDate      string   `xml:"decDate"`
	ClntBalDesc  string   `xml:"clntBalDesc"`
	DeadType     string   `xml:"deadType"`
	IdscId       string   `xml:"idscId"`
	FixDiscount  string   `xml:"fixDiscount"`
}

type BalanceDict struct {
	XMLName       xml.Name `xml:"balanceDict"`
	BalanceId     string   `xml:"balanceId"`
	BalTypeId     string   `xml:"balTypeId"`
	BalanceName   string   `xml:"balanceName"`
	DefPriority   string   `xml:"defPriority"`
	AllowCorrSub  string   `xml:"allowCorrSub"`
	AllowCorrUser string   `xml:"allowCorrUser"`
	DefFreezeDay  string   `xml:"defFreezeDay"`
	DefDeadType   string   `xml:"defDeadType"`
}

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

	match, _ := regexp.MatchString(`\*130\*99291[1597]\d{6}\*[01]#`, "*130*992911003819*1#")
	fmt.Println(match, "qweqeqe")

	url := "http://172.28.199.141:9080/inv/api/schema/invoicemgmt/DSIInvoiceManagement"
	payload := []byte(`<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/" xmlns:inv="https://www.bercut.com/inv/api/schema/invoicemgmt">
	  <SOAP-ENV:Header/>
	  <SOAP-ENV:Body>
		<inv:getClntBalance>
		  <iContract>
			<inContractSign>by_bal_clnt_id</inContractSign>
			<outContractName>base</outContractName>
			<userLogin>SCRUM-SERVICES</userLogin>
			<clntBalType>
			  <clntId>716512</clntId>
			</clntBalType>
		  </iContract>
		</inv:getClntBalance>
	  </SOAP-ENV:Body>
	</SOAP-ENV:Envelope>`)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("X-Real-User", "SCRUM-SERVICES")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return
	}

	var envelope Envelope
	err = xml.Unmarshal(body, &envelope)
	if err != nil {
		fmt.Println("Error unmarshalling response:", err)
		return
	}

	fmt.Println(envelope.Body.GetClntBalanceResponse.Return.ResultCode)
	fmt.Println(envelope.Body.GetClntBalanceResponse.Return.ClntBalance.ClientBalance.ClntId)
	fmt.Println(envelope.Body.GetClntBalanceResponse.Return.ClntBalance.ClientBalance.BalanceSum)

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

	refusalRequest, _ := regexp.MatchString(`\*130\*99291[1597]\d{6}\*0#`, req.Message)
	if refusalRequest == true {
		msisdnB := req.MSISDN
		msisdnA := req.Message[5 : len(req.Message)-3]
		err := refusalSubB(msisdnB, msisdnA)
		if err != nil {
			return
		}
		fmt.Println("Попал в отказ ")
	}

	sendToAccept, _ := regexp.MatchString(`\*130\*99291[1597]\d{6}\*1#`, req.Message)
	if sendToAccept == true {
		msisdnA := req.MSISDN
		msisdnB := req.Message[5 : len(req.Message)-3]
		err := addOrUpdateSubscriberA(msisdnA, msisdnB)
		if err != nil {
			return
		}
		fmt.Println("Попал в подтверждение ")

	}

	acceptRequest, _ := regexp.MatchString(`\*130\*99291[1597]\d{6}\*2#`, req.Message)
	if acceptRequest == true {
		msisdnB := req.MSISDN
		msisdnA := req.Message[5 : len(req.Message)-3]
		err := acceptSubB(msisdnB, msisdnA)
		if err != nil {
			return
		}
		fmt.Println("Попал в подтверждение ")
	}

	checkBalance, _ := regexp.MatchString(`\*130\*99291[1597]\d{6}#`, req.Message)
	if checkBalance == true {
		fmt.Println("Попал в проверку")
	}

}

func addOrUpdateSubscriberA(msisdnA string, msisdnB string) error {
	var subscriberAId int
	row := db.QueryRow("SELECT id FROM subscriber_a WHERE number=$1", msisdnA)

	if err := row.Scan(&subscriberAId); err != nil {
		if err == sql.ErrNoRows {
			if _, err := db.Exec("INSERT INTO subscriber_a (number) VALUES ($1)", msisdnA); err != nil {
				return fmt.Errorf("ошибка добавления номера в базу данных: %v", err)
			}
			log.Println("Номер абонента A успешно добавлен в базу данных")
			row = db.QueryRow("SELECT id FROM subscriber_a WHERE number=$1", msisdnA)
			if err := row.Scan(&subscriberAId); err != nil {
				return fmt.Errorf("ошибка получения id абонента A: %v", err)
			}
		} else {
			return fmt.Errorf("ошибка выполнения запроса: %v", err)
		}
	} else {
		log.Println("Номер абонента A существует", subscriberAId)
	}

	err := addOrUpdateSubscriberB(msisdnB, subscriberAId)
	if err != nil {
		return err
	}

	return nil
}

func addOrUpdateSubscriberB(msisdnB string, subscriberAId int) error {
	fmt.Println("suiside", subscriberAId)

	var count int
	err := db.QueryRow("SELECT count(*) FROM service WHERE subscriber_b_number=$1", msisdnB).Scan(&count)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %v", err)
	}

	if count > 0 {
		log.Println("Номер абонента B существует")
	} else {
		row := db.QueryRow("SELECT subscriber_b_number FROM service WHERE subscriber_b_number=$1", msisdnB)
		var subsNumberBFromDB string
		if err := row.Scan(&subsNumberBFromDB); err != nil {
			if err == sql.ErrNoRows {
				fmt.Println(msisdnB)
				fmt.Println("qweqw", subscriberAId)
				if _, err := db.Exec("INSERT INTO service (subscriber_a_id, subscriber_b_number, is_accept, status_id) VALUES ($1, $2, $3, 1)", subscriberAId, msisdnB, false); err != nil {

					return fmt.Errorf("ошибка добавления номера в базу данных: %v", err)
				}
				log.Println("Номер абонента B успешно добавлен в базу данных")
			} else {
				return fmt.Errorf("ошибка выполнения запроса: %v", err)
			}
		} else {
			log.Println("Номер абонента B существует")
		}
	}

	return nil
}

func acceptSubB(msisdnB, msisdnA string) error {

	var id, count int
	var numberA, numberB string
	var isAccept bool
	fmt.Println(msisdnA, " ", msisdnB)
	err := db.QueryRow("select count(*) from service s join subscriber_a sa on sa.id = s.subscriber_a_id where s.is_accept = false and s.subscriber_b_number = $1 and sa.number = $2", msisdnB, msisdnA).Scan(&count)
	fmt.Println(count)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	if count > 0 {
		err := db.QueryRow("select s.id, sa.number as subscriber_a_number, s.is_accept, s.subscriber_b_number from service s join subscriber_a sa on sa.id = s.subscriber_a_id where s.is_accept = false and s.subscriber_b_number = $1 and sa.number = $2", msisdnB, msisdnA).Scan(&id, &numberA, &isAccept, &numberB)
		fmt.Println("start update")

		if err != nil {
			return err
		}

		_, err = db.Query("update service set is_accept = true, status_id=2 where id=$1", id)
		fmt.Println("end update")

		if err != nil {
			return err
		}

		fmt.Println(id, isAccept, numberA, numberB)
	} else {
		return fmt.Errorf("Ошибка ")

	}
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %v", err)
	}

	return nil
}

func refusalSubB(msisdnB, msisdnA string) error {

	var id, count int
	var numberA, numberB string
	var isAccept bool
	fmt.Println(msisdnA, " ", msisdnB)
	err := db.QueryRow("select count(*) from service s join subscriber_a sa on sa.id = s.subscriber_a_id where s.subscriber_b_number = $1 and sa.number = $2", msisdnB, msisdnA).Scan(&count)
	fmt.Println(count)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	if count > 0 {
		err := db.QueryRow("select s.id, sa.number as subscriber_a_number, s.is_accept, s.subscriber_b_number from service s join subscriber_a sa on sa.id = s.subscriber_a_id where  s.subscriber_b_number = $1 and sa.number = $2", msisdnB, msisdnA).Scan(&id, &numberA, &isAccept, &numberB)

		if err != nil {
			return err
		}
		fmt.Println("start update")

		_, err = db.Exec("update service set is_accept = false, status_id =3  where id=$1", id)
		fmt.Println("end update")

		if err != nil {
			return err
		}

		fmt.Println(id, isAccept, numberA, numberB)
	} else {
		return fmt.Errorf("Ошибка ")

	}
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %v", err)
	}

	return nil
}
