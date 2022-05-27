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
	"database/sql"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "database-account-operator/api/v1"
)

// PostgreSQLAccountReconciler reconciles a PostgreSQLAccount object
type PostgreSQLAccountReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	DBClients       *map[string]*sql.DB
	previousAccount *v1.PostgreSQLAccountSpec
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgreSQLAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.PostgreSQLAccount{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=database-account-operator.my.domain,resources=postgresqlaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database-account-operator.my.domain,resources=postgresqlaccounts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database-account-operator.my.domain,resources=postgresqlaccounts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *PostgreSQLAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	accountApiResource := &v1.PostgreSQLAccount{}

	r.Get(ctx, req.NamespacedName, accountApiResource)

	accountSpec := accountApiResource.Spec
	accountStatus := accountApiResource.Status
	dbNamespacedName := types.NamespacedName{Name: accountSpec.PostgreSQLDatabaseName, Namespace: req.Namespace}

	var e error
	if err := validateAccount(&accountSpec); err != nil {
		e = err
	} else if (*r.DBClients)[dbNamespacedName.String()] == nil {
		e = fmt.Errorf("unable to find db client for PostgreSQLDatabase, is there a PostgreSQLDatabase api resource with name %s in ready status?", dbNamespacedName.String())
	} else if err = r.upsertAccount(&dbNamespacedName, &accountSpec); err != nil {
		e = err
	} else {
		r.previousAccount = &accountSpec
	}
	accountStatus.Ready = e == nil
	if e != nil {
		accountStatus.Error = e.Error()
	} else {
		accountStatus.Error = ""
	}
	r.Status().Update(ctx, accountApiResource)
	l.Info("Reconciled", "req", req, "account", accountSpec, "status", accountStatus)

	return ctrl.Result{}, e
}

func (r *PostgreSQLAccountReconciler) upsertAccount(namespacedName *types.NamespacedName, account *v1.PostgreSQLAccountSpec) error {
	validUntil, err := r.readValidUntil(namespacedName, account)
	if err != nil {
		return err
	}
	if validUntil == nil {
		return r.createAccount(namespacedName, account)
	}
	if validUntil != &account.ValidUntil || r.previousAccount.Password != account.Password {
		return r.updateAccount(namespacedName, account)
	}
	return nil
}

func (r *PostgreSQLAccountReconciler) updateAccount(namespacedName *types.NamespacedName, account *v1.PostgreSQLAccountSpec) error {
	query := fmt.Sprintf(`ALTER USER %s WITH PASSWORD '%s'`, account.Name, account.Password)

	if account.ValidUntil != "" {
		query = fmt.Sprintf("%s VALID UNTIL '%s'", query, account.ValidUntil)
	}

	rows, err := (*r.DBClients)[namespacedName.String()].Query(query)
	if err != nil {
		return fmt.Errorf(`error executing query ALTER USER ... for account %s : %w`, account.Name, err)
	}
	rows.Close()
	return nil
}

func (r *PostgreSQLAccountReconciler) readValidUntil(namespacedName *types.NamespacedName, account *v1.PostgreSQLAccountSpec) (*string, error) {
	query := `SELECT valuntil FROM pg_catalog.pg_user WHERE usename = $1`
	rows, err := (*r.DBClients)[namespacedName.String()].Query(query, account.Name)
	if err != nil {
		return nil, fmt.Errorf(`error executing query %s for account %s : %w`, query, account.Name, err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf(`error iterating configuration from db for account %s : %w`, account.Name, err)
	}
	var result string
	err = rows.Scan(&result)
	if err != nil {
		return nil, fmt.Errorf(`error reading configuration from db for account %s : %w`, account.Name, err)
	}
	return &result, nil
}

func (r *PostgreSQLAccountReconciler) createAccount(namespacedName *types.NamespacedName, account *v1.PostgreSQLAccountSpec) error {

	query := fmt.Sprintf(`CREATE USER %s WITH PASSWORD '%s'`, account.Name, account.Password)
	if account.ValidUntil != "" {
		query = fmt.Sprintf("%s VALID UNTIL '%s'", query, account.ValidUntil)
	}

	rows, err := (*r.DBClients)[namespacedName.String()].Query(query)
	if err != nil {
		return fmt.Errorf(`error executing query %s for account %s : %w`, query, account.Name, err)
	}
	rows.Close()
	return nil
}

func validateAccount(spec *v1.PostgreSQLAccountSpec) error {
	if !validPostgresName(spec.Name) {
		return fmt.Errorf(`invalid name %s`, spec.Name)
	}
	if !validDate(spec.ValidUntil) {
		return fmt.Errorf(`invalid date valid_until %s`, spec.ValidUntil)
	}
	return nil
}

func validDate(date string) bool {
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}
