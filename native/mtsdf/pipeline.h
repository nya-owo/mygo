#pragma once
#include <cmath>
#include <algorithm>
#include <vector>
#include "msdfgen/msdfgen.h"
#include "resolve-shape-geometry.h"
#include "precision-cleanup.h"

// ABI1 data layout is unchanged. Revision4 changes only native generation.
struct Segment {unsigned kind;float x0,y0,x1,y1,x2,y2;};
struct Result {unsigned width,height;float left,bottom,right,top;};
static_assert(sizeof(Segment)==28 && sizeof(Result)==24);

static int renderBoundaryMTSDF(const Segment *data,unsigned count,unsigned fill,unsigned char *out,unsigned capacity,Result *result){
 using namespace msdfgen;
 if(!data||!result||!count||count>8192||fill>1||capacity>512*512*4)return -1;
 Shape shape;Contour *contour=nullptr;Point2 pen,start;
 for(unsigned i=0;i<count;i++){
  const Segment &s=data[i];const float *v=&s.x0;for(int k=0;k<6;k++)if(!std::isfinite(v[k])||fabs(v[k])>16)return -2;
  Point2 p0(s.x0,-s.y0),p1(s.x1,-s.y1),p2(s.x2,-s.y2);
  switch(s.kind){case 0:if(contour)return -2;contour=&shape.addContour();pen=start=p0;break;
   case 1:if(!contour)return -2;if(pen!=p0)contour->addEdge(EdgeHolder(pen,p0));pen=p0;break;
   case 3:if(!contour)return -2;contour->addEdge(EdgeHolder(pen,p0,p1,p2));pen=p2;break;
   case 4:if(!contour)return -2;if(pen!=start)contour->addEdge(EdgeHolder(pen,start));contour=nullptr;break;
   default:return -2;
  }
 }
 if(contour||!shape.validate()||shape.edgeCount()==0)return -2;
 shape.normalize();auto bounds=shape.getBounds();double l=floor(bounds.l*128)-4,b=floor(bounds.b*128)-4,r=ceil(bounds.r*128)+4,t=ceil(bounds.t*128)+4;
 unsigned w=(unsigned)(r-l),h=(unsigned)(t-b);if(!w||!h||w>512||h>512)return -4;
 *result={w,h,float((l+.5)/128),float((b+.5)/128),float((r-.5)/128),float((t-.5)/128)};
 // The sizing call avoids Boolean work and generation. Original metrics/UV
 // sample centers remain identical to the accepted candidate.
 if(!out||capacity<w*h*4)return 1;
 if(!resolveShapeGeometryWithFill(shape,fill==0))return -5;
 double maxMove;collapseFloatZeroEdges(shape,maxMove);shape.normalize();
 if(!shape.validate()||shape.edgeCount()==0)return -5;
 edgeColoringInkTrap(shape,3.0,0);
 Projection projection(Vector2(128),Vector2(-l/128,-b/128));SDFTransformation transformation(projection,Range(6./128));
 MSDFGeneratorConfig config(true);config.errorCorrection.mode=ErrorCorrectionConfig::DISABLED;
 Bitmap<float,4> bitmap(w,h);generateMTSDF(bitmap,shape,transformation,config);
 distanceSignCorrection(bitmap,shape,projection,.5f,fill==0?FILL_ODD:FILL_NONZERO);
 MSDFGeneratorConfig post(true);post.errorCorrection.distanceCheckMode=ErrorCorrectionConfig::DO_NOT_CHECK_DISTANCE;
 msdfErrorCorrection(bitmap,shape,transformation,post);
 for(unsigned y=0;y<h;y++)for(unsigned x=0;x<w;x++)for(int k=0;k<4;k++){
  float value=bitmap(x,y)[k];if(!std::isfinite(value))return -6;
  out[((h-1-y)*w+x)*4+k]=(unsigned char)(std::clamp(value,0.f,1.f)*255.f+.5f);
 }
 return 0;
}
