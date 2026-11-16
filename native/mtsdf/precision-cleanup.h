#pragma once
#include <algorithm>
#include <cmath>
#include <cstddef>
#include <limits>
#include "msdfgen/core/Shape.h"
// DirectWrite and Skia path coordinates are float32. Collapse only numerically
// zero edges with max control-point distance from p0 <= epsilon, where
// epsilon = 4 * FLT_EPSILON * max(1, maxAbsBounds).
// Preserve the adjacent curve's initial tangent while welding the shared vertex.
static int collapseFloatZeroEdges(msdfgen::Shape &shape,double &maxMove) {
 using namespace msdfgen;int removed=0;maxMove=0;
 auto bounds=shape.getBounds();double extent=std::max({1.,fabs(bounds.l),fabs(bounds.b),fabs(bounds.r),fabs(bounds.t)});
 double epsilon=4*std::numeric_limits<float>::epsilon()*extent;
 for(auto &contour:shape.contours){
  for(size_t i=0;!contour.edges.empty()&&i<contour.edges.size();){
   const EdgeSegment *edge=contour.edges[i];const Point2 *p=edge->controlPoints();double maxControlPointDistanceFromStart=0;for(int j=1;j<=edge->type();j++)maxControlPointDistanceFromStart=std::max(maxControlPointDistanceFromStart,(p[j]-p[0]).length());
   if(maxControlPointDistanceFromStart>epsilon){++i;continue;}
   if(contour.edges.size()==1){contour.edges.clear();removed++;break;}
   size_t next=(i+1)%contour.edges.size();EdgeSegment *n=contour.edges[next];Point2 newStart=p[0];Point2 oldStart=n->point(0);Vector2 delta=newStart-oldStart;maxMove=std::max(maxMove,delta.length());
   switch(n->type()){
    case 1:static_cast<LinearSegment*>(n)->p[0]=newStart;break;
    case 2:{auto *q=static_cast<QuadraticSegment*>(n);q->p[0]=newStart;q->p[1]+=delta;break;}
    case 3:{auto *q=static_cast<CubicSegment*>(n);q->p[0]=newStart;q->p[1]+=delta;break;}
   }
   contour.edges.erase(contour.edges.begin()+i);removed++;
  }
 }
 shape.contours.erase(std::remove_if(shape.contours.begin(),shape.contours.end(),[](const Contour &c){return c.edges.empty();}),shape.contours.end());
 return removed;
}
