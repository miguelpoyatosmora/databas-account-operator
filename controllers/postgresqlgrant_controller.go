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
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PostgreSQLGrantReconciler reconciles a PostgreSQLGrant object
type PostgreSQLGrantReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	DBClients     *map[string]*sql.DB
	previousGrant *v1.PostgreSQLGrantSpec
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgreSQLGrantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.PostgreSQLGrant{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=database-account-operator.my.domain,resources=postgresqlgrants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database-account-operator.my.domain,resources=postgresqlgrants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database-account-operator.my.domain,resources=postgresqlgrants/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *PostgreSQLGrantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	grantApiResource := &v1.PostgreSQLGrant{}

	r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, grantApiResource)

	grantSpec := grantApiResource.Spec
	grantStatus := grantApiResource.Status
	dbNamespacedName := types.NamespacedName{Name: grantSpec.PostgreSQLDatabaseName, Namespace: req.Namespace}

	var e error
	if err := validateGrantSpec(&grantSpec); err != nil {
		e = err
	} else if (*r.DBClients)[dbNamespacedName.String()] == nil {

		e = fmt.Errorf("unable to find db client for PostgreSQLDatabase, is there a PostgreSQLDatabase api resource with name %s in ready status?", dbNamespacedName.String())
	} else if err = r.upsertSchema(&dbNamespacedName, grantSpec.Schema); err != nil {
		e = err
	} else if err = r.upsertGrant(&dbNamespacedName, &grantSpec); err != nil {
		e = err
	} else {
		r.previousGrant = &grantSpec
	}

	grantStatus.Ready = e == nil
	if e != nil {
		grantStatus.Error = e.Error()
	} else {
		grantStatus.Error = ""
	}
	r.Status().Update(ctx, grantApiResource)
	log.FromContext(ctx).Info("Reconciled", "req", req, "grant", grantSpec, "status", grantStatus)
	return ctrl.Result{}, e
}

func (r *PostgreSQLGrantReconciler) upsertSchema(dbNamespacedName *types.NamespacedName, schema string) error {
	exists, err := r.schemaExists(dbNamespacedName, schema)
	if err != nil {
		return err
	}
	if !exists {
		return r.createSchema(dbNamespacedName, schema)
	}
	return nil
}

func (r *PostgreSQLGrantReconciler) schemaExists(dbNamespacedName *types.NamespacedName, schema string) (bool, error) {
	query := `SELECT schema_name FROM information_schema.schemata WHERE schema_name = $1;`
	rows, err := (*r.DBClients)[dbNamespacedName.String()].Query(query, strings.ToLower(schema))
	if err != nil {
		return false, fmt.Errorf(`error executing query %s for schema %s : %w`, query, schema, err)
	}
	defer rows.Close()
	if !rows.Next() {
		return false, nil
	}
	err = rows.Err()
	if err != nil {
		return false, fmt.Errorf(`error iterating configuration from db for schema %s : %w`, schema, err)
	}
	var result string
	err = rows.Scan(&result)
	if err != nil {
		return false, fmt.Errorf(`error reading configuration from db for schema %s : %w`, schema, err)
	}
	return result == schema, nil
}

func (r *PostgreSQLGrantReconciler) createSchema(dbNamespacedName *types.NamespacedName, schema string) error {
	query := fmt.Sprintf(`CREATE SCHEMA %s`, strings.ToLower(schema))

	rows, err := (*r.DBClients)[dbNamespacedName.String()].Query(query)
	if err != nil {
		return fmt.Errorf(`error executing query %s for schema %s : %w`, query, schema, err)
	}
	rows.Close()
	return nil
}

func (r *PostgreSQLGrantReconciler) upsertGrant(dbNamespacedName *types.NamespacedName, grantSpec *v1.PostgreSQLGrantSpec) error {
	tExists, err := r.tablesExists(dbNamespacedName, grantSpec.Schema)
	if err != nil {
		return err
	}
	if !tExists {
		return nil
	}
	gExists, err := r.grantExists(dbNamespacedName, grantSpec)
	if err != nil {
		return err
	}
	if !gExists {
		return r.createGrant(dbNamespacedName, grantSpec)
	}
	return nil
}

func (r *PostgreSQLGrantReconciler) grantExists(dbNamespacedName *types.NamespacedName, grantSpec *v1.PostgreSQLGrantSpec) (bool, error) {
	query := `SELECT grantee, privilege_type FROM information_schema.role_table_grants WHERE table_schema=$1;`
	rows, err := (*r.DBClients)[dbNamespacedName.String()].Query(query, strings.ToLower(grantSpec.Schema))
	if err != nil {
		return false, fmt.Errorf(`error executing query %s for grant %+v : %w`, query, grantSpec, err)
	}
	defer rows.Close()
	if !rows.Next() {
		return false, nil
	}
	err = rows.Err()
	if err != nil {
		return false, fmt.Errorf(`error iterating configuration from db for grant %+v : %w`, grantSpec, err)
	}
	var result string
	err = rows.Scan(&result)
	if err != nil {
		return false, fmt.Errorf(`error reading configuration from db for grant %+v : %w`, grantSpec, err)
	}
	return result != "", nil
}

func (r *PostgreSQLGrantReconciler) tablesExists(dbNamespacedName *types.NamespacedName, schema string) (bool, error) {

	query := `SELECT table_schema FROM information_schema.tables WHERE table_schema = $1;`
	rows, err := (*r.DBClients)[dbNamespacedName.String()].Query(query, strings.ToLower(schema))
	if err != nil {
		return false, fmt.Errorf(`error executing query %s for schema %s : %w`, query, schema, err)
	}
	defer rows.Close()
	if !rows.Next() {
		return false, nil
	}
	err = rows.Err()
	if err != nil {
		return false, fmt.Errorf(`error iterating configuration from db for schema %s : %w`, schema, err)
	}
	var result string
	err = rows.Scan(&result)
	if err != nil {
		return false, fmt.Errorf(`error reading configuration from db for schema %s : %w`, schema, err)
	}
	return result != "", nil

}

func (r *PostgreSQLGrantReconciler) createGrant(dbNamespacedName *types.NamespacedName, grantSpec *v1.PostgreSQLGrantSpec) error {
	query := `GRANT $1 ON ALL TABLES IN SCHEMA $2 TO $3`
	rows, err := (*r.DBClients)[dbNamespacedName.String()].Query(
		query,
		strings.Join(grantSpec.Type[:], ","),
		strings.ToLower(grantSpec.Schema),
	)
	if err != nil {
		return fmt.Errorf(`error executing query %s for grant %+v : %w`, query, grantSpec, err)
	}
	rows.Close()
	return nil
}

func validateGrantSpec(spec *v1.PostgreSQLGrantSpec) error {

	if !validPostgresName(spec.Schema) {
		return fmt.Errorf(`invalid schema %s`, spec.Schema)
	}
	if !validPostgresName(spec.To) {
		return fmt.Errorf(`invalid to %s`, spec.To)
	}
	if !validGrantType(spec.Type) {
		return fmt.Errorf(`invalid grant types %v`, spec.Type)
	}
	return nil
}

func validGrantType(types []string) bool {
	for _, t := range types {
		switch strings.ToLower(t) {
		case "select", "insert", "update", "delete", "truncate":
		case "all":
			if len(types) > 1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
