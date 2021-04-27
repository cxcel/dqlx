package deku

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type Operation interface {
	DQLizer
	GetName() string
}

type queryOperation struct {
	operations []Operation
	variables  []Operation
}

func OperationQuery(queries ...QueryBuilder) (query string, args map[string]interface{}, err error) {
	mainOperation := queryOperation{}

	for _, query := range queries {
		mainOperation.operations = append(mainOperation.operations, query.rootEdge)

		for _, variable := range query.variables {
			mainOperation.variables = append(mainOperation.variables, variable.rootEdge)
		}
	}

	return mainOperation.ToDQL()
}

func (grammar queryOperation) ToDQL() (query string, variables map[string]interface{}, err error) {
	blocNames := make([]string, len(grammar.operations))

	for index, block := range grammar.operations {
		blocNames[index] = strings.Title(strings.ToLower(block.GetName()))
	}

	queryName := strings.Join(blocNames, "_")

	var args []interface{}
	var statements []string

	if err := addOperation(grammar.variables, &statements, &args); err != nil {
		return "", nil, err
	}

	if err := addOperation(grammar.operations, &statements, &args); err != nil {
		return "", nil, err
	}

	innerQuery := strings.Join(statements, " ")

	query, variables, err = replacePlaceholders(innerQuery, args)

	if err != nil {
		return
	}

	queryPlaceholderNames := getSortedVariables(variables)
	placeholders := make([]string, len(queryPlaceholderNames))

	for index, placeholderName := range queryPlaceholderNames {
		placeholders[index] = fmt.Sprintf("$%s:%s", placeholderName, goTypeToDQLType(variables[placeholderName]))
	}

	writer := bytes.Buffer{}
	writer.WriteString(fmt.Sprintf("query %s(%s) {", queryName, strings.Join(placeholders, ", ")))
	writer.WriteString(" " + query)
	writer.WriteString(" }")

	return writer.String(), variables, nil
}

func goTypeToDQLType(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int8, int32, int64:
		return "int"
	case float32, float64:
		return "float"
	case bool:
		return "bool"
	case time.Time, *time.Time:
		return "datetime"
	}

	return "string"
}

func replacePlaceholders(query string, args []interface{}) (string, map[string]interface{}, error) {
	variables := map[string]interface{}{}
	buf := &bytes.Buffer{}
	i := 0

	for {
		p := strings.Index(query, "??")
		if p == -1 {
			break
		}

		buf.WriteString(query[:p])
		key := fmt.Sprintf("%d", i)
		buf.WriteString("$" + key)
		query = query[p+2:]

		// Assign the variables
		variables[key] = args[i]

		i++
	}

	buf.WriteString(query)
	return buf.String(), variables, nil
}

func isListType(val interface{}) bool {
	valVal := reflect.ValueOf(val)
	return valVal.Kind() == reflect.Array || valVal.Kind() == reflect.Slice
}

func addOperation(operations []Operation, statements *[]string, args *[]interface{}) error {
	parts := make([]DQLizer, len(operations))

	for index, operation := range operations {
		parts[index] = operation
	}

	return addStatement(parts, statements, args)
}

func addStatement(parts []DQLizer, statements *[]string, args *[]interface{}) error {
	for _, block := range parts {
		statement, queryArg, err := block.ToDQL()

		if err != nil {
			return err
		}

		*statements = append(*statements, statement)
		*args = append(*args, queryArg...)
	}

	return nil
}

func addPart(part DQLizer, writer *bytes.Buffer, args *[]interface{}) error {
	statement, statementArgs, err := part.ToDQL()
	*args = append(*args, statementArgs...)

	if err != nil {
		return err
	}

	writer.WriteString(statement)

	return nil
}