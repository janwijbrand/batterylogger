# batterijtje enclosure — render/export driver.
#   make views   fast PNGs of every view (this is the tight iteration loop)
#   make stl     both printable parts, ready to slice
#   make test    clearance assertions: any output geometry is a fit bug

SRC   := enclosure.scad params.scad boards.scad
SCAD  := /Applications/OpenSCAD.app/Contents/MacOS/OpenSCAD
OUT   := build
IMG   := --imgsize=1100,850 --colorscheme=Tomorrow
CTR   := 47.5,33,9

# camera: translate(3) rotate(3) distance
CAM_ISO   := $(CTR),58,0,20,300
CAM_ISOB  := $(CTR),122,0,205,300
CAM_FRONT := $(CTR),0,0,0,260
CAM_BACK  := $(CTR),180,0,0,260
CAM_EDGE  := $(CTR),90,0,0,260

.PHONY: views stl gauge test clean
views: $(OUT)/assembly-iso.png $(OUT)/assembly-front.png $(OUT)/tray-iso.png \
       $(OUT)/tray-back.png $(OUT)/bezel-iso.png $(OUT)/bezel-front.png \
       $(OUT)/section.png

$(OUT)/assembly-iso.png: $(SRC) ; $(SCAD) -o $@ $(IMG) --camera=$(CAM_ISO)   -D 'part="assembly"' enclosure.scad
$(OUT)/assembly-front.png: $(SRC) ; $(SCAD) -o $@ $(IMG) --camera=$(CAM_FRONT) --projection=o -D 'part="assembly"' enclosure.scad
$(OUT)/tray-iso.png: $(SRC) ; $(SCAD) -o $@ $(IMG) --camera=$(CAM_ISO)   -D 'part="tray"'  enclosure.scad
$(OUT)/tray-back.png: $(SRC) ; $(SCAD) -o $@ $(IMG) --camera=$(CAM_ISOB)  -D 'part="tray"'  enclosure.scad
$(OUT)/bezel-iso.png: $(SRC) ; $(SCAD) -o $@ $(IMG) --camera=$(CAM_ISOB)  -D 'part="bezel"' enclosure.scad
$(OUT)/bezel-front.png: $(SRC) ; $(SCAD) -o $@ $(IMG) --camera=$(CAM_FRONT) --projection=o -D 'part="bezel"' enclosure.scad
$(OUT)/section.png: $(SRC) ; $(SCAD) -o $@ $(IMG) --camera=$(CAM_EDGE)  -D 'part="section"' enclosure.scad

stl: $(OUT)/tray.stl $(OUT)/bezel.stl
gauge: $(OUT)/gauge_front.stl $(OUT)/gauge_floor.stl $(OUT)/gauge-front.png $(OUT)/gauge-floor.png
$(OUT)/gauge_front.stl: $(SRC) ; $(SCAD) -o $@ --export-format=binstl -D 'part="gauge_front"' enclosure.scad
$(OUT)/gauge_floor.stl: $(SRC) ; $(SCAD) -o $@ --export-format=binstl -D 'part="gauge_floor"' enclosure.scad
$(OUT)/gauge-front.png: $(SRC) ; $(SCAD) -o $@ $(IMG) --camera=$(CAM_ISO) -D 'part="gauge_front"' enclosure.scad
$(OUT)/gauge-floor.png: $(SRC) ; $(SCAD) -o $@ $(IMG) --camera=$(CAM_ISO) -D 'part="gauge_floor"' enclosure.scad
$(OUT)/tray.stl: $(SRC) ; $(SCAD) -o $@ --export-format=binstl -D 'part="tray"'  enclosure.scad
$(OUT)/bezel.stl: $(SRC) ; $(SCAD) -o $@ --export-format=binstl -D 'part="bezel"' enclosure.scad

# Each test renders the overlap between a printed part and a grown board mock.
# Empty output (0 facets) = pass.
test:
	@for t in $$(seq 0 7); do \
	  r=$$($(SCAD) -o $(OUT)/t.stl --export-format=binstl --summary geometry --summary-file - \
	       -D "test=$$t" tests.scad 2>/dev/null | sed -n 's/.*"facets":\([0-9]*\).*/\1/p'); \
	  n=$$($(SCAD) -o /dev/null --export-format=binstl -D "test=$$t" -D 'name_only=true' tests.scad 2>&1 | sed -n 's/^ECHO: "\(.*\)"$$/\1/p' | head -1); \
	  if [ "$${r:-0}" = "0" ]; then echo "  PASS  $$t $$n"; else echo "  FAIL  $$t $$n  ($$r facets of overlap)"; fi; \
	done

# make dbg T=0  -> look at what test 0 is complaining about
dbg:
	$(SCAD) -o $(OUT)/dbg.png $(IMG) --camera=$(CAM_ISO) -D "test=$(T)" -D 'debug=true' tests.scad

clean: ; rm -f $(OUT)/*.png $(OUT)/*.stl
