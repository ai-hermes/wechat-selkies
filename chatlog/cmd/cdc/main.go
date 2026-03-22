package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sjzar/chatlog/internal/chatlog/ctx"
)

func main() {
	context, loadCtxErr := ctx.New("")
	if loadCtxErr != nil {
		panic(loadCtxErr)
	}
	fmt.Printf("%+v\n", context)

	//syncType := "user" // user or chatroom
	//syncUserId := "wxid_wg5luiy2hc8o22"

	syncType := "chatroom" // user or chatroom
	syncUserId := "46087212284@chatroom"

	// 计算md5值
	hash := md5.Sum([]byte(syncUserId))
	md5Value := hex.EncodeToString(hash[:])

	var targetSeqList string
	var err error

	// get seq from source table
	sourceTablePath := "/Users/dingwenjiang/workspace/codereview/warjiang/wxapp/db_storage/message/message_0.db"
	/*
		select GROUP_CONCAT(sort_seq, ',') as seq_list from Msg_cb2835a3d5d14c40370b08be7211263d;
	*/
	sourceSeqList, err := querySourceSeqList(sourceTablePath, md5Value)
	if err != nil {
		fmt.Printf("Failed to query source seq list: %v\n", err)
		return
	}
	fmt.Printf("Source seq list: %s\n", sourceSeqList)

	// get seq from target table
	targetTablePath := "/Users/dingwenjiang/Library/Application Support/rethink-ai/wechat-mem0-chats.db"
	targetSeqList, err = queryTargetSeqList(targetTablePath, syncType, syncUserId)
	if err != nil {
		fmt.Printf("Failed to query target seq list: %v\n", err)
		return
	}
	fmt.Printf("Target seq list: %s\n", targetSeqList)

	// diff seq list
	sourceSeqs := parseSeqList(sourceSeqList)
	targetSeqs := parseSeqList(targetSeqList)

	incrementalSeqs := calculateIncrementalSeqs(sourceSeqs, targetSeqs)
	fmt.Printf("Incremental seqs (count: %d): %v\n", len(incrementalSeqs), incrementalSeqs)
}

func parseSeqList(seqList string) []int64 {
	if seqList == "" {
		return []int64{}
	}

	var seqs []int64
	var currentNum int64
	isBuildingNum := false

	for i := 0; i < len(seqList); i++ {
		char := seqList[i]
		if char >= '0' && char <= '9' {
			currentNum = currentNum*10 + int64(char-'0')
			isBuildingNum = true
		} else if char == ',' && isBuildingNum {
			seqs = append(seqs, currentNum)
			currentNum = 0
			isBuildingNum = false
		}
	}

	if isBuildingNum {
		seqs = append(seqs, currentNum)
	}

	return seqs
}

func calculateIncrementalSeqs(sourceSeqs, targetSeqs []int64) []int64 {
	targetSet := make(map[int64]struct{})
	for _, seq := range targetSeqs {
		targetSet[seq] = struct{}{}
	}

	var incrementalSeqs []int64
	for _, seq := range sourceSeqs {
		if _, exists := targetSet[seq]; !exists {
			incrementalSeqs = append(incrementalSeqs, seq)
		}
	}

	return incrementalSeqs
}

func querySourceSeqList(dbPath, tableMD5 string) (string, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return "", err
	}
	defer db.Close()

	tableName := fmt.Sprintf("Msg_%s", tableMD5)

	if !isValidTableName(tableName) {
		return "", fmt.Errorf("invalid table name: %s", tableName)
	}

	query := fmt.Sprintf("SELECT GROUP_CONCAT(sort_seq, ',') as seq_list FROM %s", tableName)

	var seqList sql.NullString
	err = db.QueryRow(query).Scan(&seqList)
	if err != nil {
		return "", err
	}

	if seqList.Valid {
		return seqList.String, nil
	}
	return "", nil
}

func isValidTableName(tableName string) bool {
	if len(tableName) == 0 || len(tableName) > 100 {
		return false
	}

	for _, char := range tableName {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_') {
			return false
		}
	}

	return true
}

func queryTargetSeqList(dbPath, sync_type string, sender string) (string, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return "", err
	}
	defer db.Close()

	query := `
		SELECT GROUP_CONCAT(seq, ',') AS seq_list
		FROM wechat_message
		WHERE is_chat_room = 0
		  AND sender = ?
		ORDER BY seq
	`
	if sync_type == "chatroom" {
		query = `
			SELECT GROUP_CONCAT(seq, ',') AS seq_list
			FROM wechat_message
			WHERE is_chat_room = 1
			  AND talker= ?
			ORDER BY seq
		
		`
	}

	var seqList sql.NullString
	err = db.QueryRow(query, sender).Scan(&seqList)
	if err != nil {
		return "", err
	}

	if seqList.Valid {
		return seqList.String, nil
	}
	return "", nil
}
