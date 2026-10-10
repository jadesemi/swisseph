/* cgo compiles the .c files in package swe's own directory; this shim pulls in
   one Swiss Ephemeris source from the fork root, resolved through the binding's
   -I${SRCDIR}/.. CFLAGS. Only the seven sources the binding needs are included
   this way, so the fork-root samples carrying their own main() (swetest.c,
   swemini.c) are never compiled. */
#include "sweph.c"
