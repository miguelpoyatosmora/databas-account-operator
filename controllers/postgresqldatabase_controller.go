/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	v1 "database-account-operator/api/v1"
	"database/sql"
	"fmt"
	"regexp"

	_ "github.com/lib/pq"
	"golang.org/x/text/language"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PostgreSQLDatabaseReconciler reconciles a PostgreSQLDatabase object
type PostgreSQLDatabaseReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	DBClients      *map[string]*sql.DB
	previousDBSpec *v1.PostgreSQLDatabaseSpec
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgreSQLDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.PostgreSQLDatabase{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=database-account-operator.my.domain,resources=postgresqldatabases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database-account-operator.my.domain,resources=postgresqldatabases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database-account-operator.my.domain,resources=postgresqldatabases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *PostgreSQLDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	dbApiResource := &v1.PostgreSQLDatabase{}
	namespacedName := types.NamespacedName{Name: req.Name, Namespace: req.Namespace}
	r.Get(ctx, namespacedName, dbApiResource)
	dbSpec := dbApiResource.Spec
	dbStatus := dbApiResource.Status

	var e error
	if err := validateDatabase(&dbSpec); err != nil {
		dbStatus.Error = err.Error()
	} else if err := r.dbOpen(&namespacedName, &dbSpec); err != nil {
		dbStatus.Error = err.Error()
	} else if err = r.createDBIfNotExists(&namespacedName, &dbSpec); err != nil {
		dbStatus.Error = err.Error()
	} else {
		dbStatus.Error = ""
		r.previousDBSpec = &dbSpec
	}

	dbStatus.Ready = e == nil
	if e != nil {
		dbStatus.Error = e.Error()
	} else {
		dbStatus.Error = ""
	}
	r.Status().Update(ctx, dbApiResource)
	log.FromContext(ctx).Info("Reconciled", "req", req, "dbSpec", dbSpec, "dbStatus", dbStatus)
	return ctrl.Result{}, e
}

func (r *PostgreSQLDatabaseReconciler) dbOpen(namespacedName *types.NamespacedName, dbSpec *v1.PostgreSQLDatabaseSpec) error {

	dbClient := (*r.DBClients)[namespacedName.String()]
	if dbClient == nil ||
		r.previousDBSpec == nil ||
		r.previousDBSpec.User != dbSpec.User ||
		r.previousDBSpec.Password != dbSpec.Password ||
		r.previousDBSpec.Address != dbSpec.Address {
		if dbClient != nil {
			(*r.DBClients)[namespacedName.String()] = nil
			err := dbClient.Close()
			if err != nil {
				return err
			}
		}
		//TODO: Enable ssl configuration
		connStr := fmt.Sprintf("postgresql://%s:%s@%s?sslmode=disable", dbSpec.User, dbSpec.Password, dbSpec.Address)
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			return err
		}
		(*r.DBClients)[namespacedName.String()] = db
	}
	return nil
}

//TODO: Make it atomic, possible solution here: https://stackoverflow.com/questions/18389124/simulate-create-database-if-not-exists-for-postgresql
// It is not critical because race conditions will be solved in the next reconcile cycle
func (r *PostgreSQLDatabaseReconciler) createDBIfNotExists(namespacedName *types.NamespacedName, dbSpec *v1.PostgreSQLDatabaseSpec) error {
	dbConf, err := r.readDBConfig(namespacedName, dbSpec.Database)
	if err != nil {
		return err
	}
	if dbConf == nil {
		return r.createDB(namespacedName, dbSpec)
	}
	if dbSpec.Encoding != "" && dbConf.encoding != dbSpec.Encoding {
		return fmt.Errorf("database %s current encoding is %s but desired encoding %s, please backup and delete manually the existing database",
			dbSpec.Database, dbConf.encoding, dbSpec.Encoding)
	}
	if dbSpec.LC_Collate != "" && dbConf.collate != dbSpec.LC_Collate {
		return fmt.Errorf("database %s current LC_Collate is %s but desired LC_Collate %s, please backup and delete manually the existing database",
			dbSpec.Database, dbConf.collate, dbSpec.LC_Collate)
	}
	if dbSpec.LC_CType != "" && dbConf.ctype != dbSpec.LC_CType {
		return fmt.Errorf("database %s current LC_CType is %s but desired LC_CType %s, please backup and delete manually the existing database",
			dbSpec.Database, dbConf.ctype, dbSpec.LC_CType)
	}
	return nil
}

func (r *PostgreSQLDatabaseReconciler) createDB(namespacedName *types.NamespacedName, dbSpec *v1.PostgreSQLDatabaseSpec) error {
	//create database does not support parameters
	query := fmt.Sprintf(`CREATE DATABASE %s`, dbSpec.Database)
	if dbSpec.Encoding != "" {
		query = fmt.Sprintf("%s ENCODING '%s'", query, dbSpec.Encoding)
	}
	if dbSpec.LC_Collate != "" {
		query = fmt.Sprintf("%s LC_COLLATE '%s'", query, dbSpec.LC_Collate)
	}
	if dbSpec.LC_CType != "" {
		query = fmt.Sprintf("%s LC_CTYPE '%s'", query, dbSpec.LC_CType)
	}
	rows, err := (*r.DBClients)[namespacedName.String()].Query(query)
	if err != nil {
		return fmt.Errorf(`error executing query %s %w`, query, err)
	}
	rows.Close()
	return nil
}

func (r *PostgreSQLDatabaseReconciler) readDBConfig(namespacedName *types.NamespacedName, database string) (*dbConfig, error) {
	query := `SELECT pg_encoding_to_char(encoding),datcollate,datcollate FROM pg_database WHERE datname = $1`
	rows, err := (*r.DBClients)[namespacedName.String()].Query(query, database)
	if err != nil {
		return nil, fmt.Errorf(`error executing query %s for database %s : %w`, query, database, err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf(`error iterating configuration from db for database %s : %w`, database, err)
	}
	result := &dbConfig{}
	err = rows.Scan(&result.encoding, &result.collate, &result.ctype)
	if err != nil {
		return nil, fmt.Errorf(`error reading configuration from db for database %s : %w`, database, err)
	}
	return result, nil
}

type dbConfig struct {
	encoding, collate, ctype string
}

func validateDatabase(dbSpec *v1.PostgreSQLDatabaseSpec) error {
	if !validAddress(dbSpec.Address) {
		return fmt.Errorf(`invalid address %s`, dbSpec.Address)
	}
	if !validPostgresName(dbSpec.User) {
		return fmt.Errorf(`invalid user %s`, dbSpec.User)
	}
	if !validPostgresName(dbSpec.Database) {
		return fmt.Errorf(`invalid database name %s`, dbSpec.Database)
	}
	if !validEncoding(dbSpec.Encoding) {
		return fmt.Errorf(`invalid encoding %s`, dbSpec.Encoding)
	}
	if !validLocale(dbSpec.LC_Collate) {
		return fmt.Errorf(`invalid lc_collate %s`, dbSpec.LC_Collate)
	}
	if !validLocale(dbSpec.LC_CType) {
		return fmt.Errorf(`invalid lc_ctype %s`, dbSpec.LC_CType)
	}
	return nil
}

func validAddress(dnsPort string) bool {
	return regexAddress.Match([]byte(dnsPort))
}

func validPostgresName(name string) bool {
	return regexPostgresName.Match([]byte(name))
}

func validLocale(locale string) bool {
	_, err := language.Parse(locale)
	return err != nil
}

func validEncoding(encoding string) bool {
	for _, a := range availableEncodings {
		if a == encoding {
			return true
		}
	}
	return false
}

//TODO: support other names supported by postgres
var regexPostgresName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

var regexAddress = regexp.MustCompile(`^(?:[A-Za-z0-9-]+\.)+[A-Za-z0-9]{1,3}:\d{1,5}$`)

var availableEncodings = []string{
	"BIG5",
	"EUC_CN",
	"EUC_JP",
	"EUC_JIS_2004",
	"EUC_KR",
	"EUC_TW",
	"GB18030",
	"GBK",
	"ISO_8859_5",
	"ISO_8859_6",
	"ISO_8859_7",
	"ISO_8859_8",
	"JOHAB",
	"KOI8R",
	"KOI8U",
	"LATIN1",
	"LATIN2",
	"LATIN3",
	"LATIN4",
	"LATIN5",
	"LATIN6",
	"LATIN7",
	"LATIN8",
	"LATIN9",
	"LATIN10",
	"MULE_INTERNAL",
	"SJIS",
	"SHIFT_JIS_2004",
	"SQL_ASCII",
	"UHC",
	"UTF8",
	"WIN866",
	"WIN874",
	"WIN1250",
	"WIN1251",
	"WIN1252",
	"WIN1253",
	"WIN1254",
	"WIN1255",
	"WIN1256",
	"WIN1257",
	"WIN1258",
}
