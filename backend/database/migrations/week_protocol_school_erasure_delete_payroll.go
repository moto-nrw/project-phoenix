package migrations

// Payroll dispatches are erased before their school and its monthly snapshots.
const weekProtocolPayrollErasureDeleteSQL = `
   SELECT count(*) INTO v_count FROM active.payroll_dispatches AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.payroll_dispatches AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.payroll_dispatches', v_count);`
