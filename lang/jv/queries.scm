;; depusage queries for Java imports.
;;
;; Java import_declaration shapes:
;;   import com.foo.Bar;
;;   import com.foo.Bar.method;       // static
;;   import com.foo.*;
;;
;; We capture the declaration; Go code reads its scoped_identifier.
(import_declaration) @import_decl
